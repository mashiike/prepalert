package prepalert

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/hashicorp/hcl/v2"
	"github.com/kayac/go-katsubushi"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/canyon"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/mashiike/queryrunner"
	"github.com/mashiike/slogutils"
)

type App struct {
	mkrSvc              *MackerelService
	backend             Backend
	webhookClientID     string
	webhookClientSecret string
	rules               []*Rule
	queueName           string
	queueUrl            string
	sqsClient           *sqs.Client
	evalCtx             *hcl.EvalContext
}

func New(apikey string, cfg *hclconfig.Config) (*App, error) {
	return NewWithMackerelClient(mackerel.NewClient(apikey), cfg)
}

func NewWithMackerelClient(client MackerelClient, cfg *hclconfig.Config) (*App, error) {
	svc := NewMackerelService(client)
	var backend Backend
	switch {
	case !cfg.Prepalert.S3Backend.IsEmpty():
		awsCfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			return nil, fmt.Errorf("load aws default config:%w", err)
		}
		s3Client := s3.NewFromConfig(awsCfg)
		backend, err = NewS3Backend(s3Client, cfg.Prepalert.S3Backend, cfg.Prepalert.Auth)
		if err != nil {
			return nil, fmt.Errorf("initialize s3 backend:%w", err)
		}
	default:
		backend = NewDiscardBackend()
	}
	slog.Info("setup backend", "backend", backend.String())

	rules := make([]*Rule, 0, len(cfg.Rules))
	for i, ruleBlock := range cfg.Rules {
		rule, err := NewRule(svc, backend, ruleBlock, cfg.Prepalert.Service)
		if err != nil {
			return nil, fmt.Errorf("rules[%d]:%w", i, err)
		}
		rules = append(rules, rule)
	}
	app := &App{
		mkrSvc:    svc,
		backend:   backend,
		rules:     rules,
		queueName: cfg.Prepalert.SQSQueueName,
		evalCtx:   cfg.EvalContext,
	}
	if !cfg.Prepalert.Auth.IsEmpty() {
		app.webhookClientID = cfg.Prepalert.Auth.ClientID
		app.webhookClientSecret = cfg.Prepalert.Auth.ClientSecret
	}
	return app, nil
}

type RunOptions struct {
	Mode      string `help:"run mode" env:"PREPALERT_MODE" default:"http"`
	Address   string `help:"run local address" env:"PREPALERT_ADDRESS" default:":8080"`
	Prefix    string `help:"run server prefix" env:"PREPALERT_PREFIX" default:"/"`
	BatchSize int    `help:"run local sqs batch size" env:"PREPALERT_BATCH_SIZE" default:"1"`
}

func (app *App) Run(ctx context.Context, opts *RunOptions) error {
	canyonOpts := []canyon.Option{
		canyon.WithCanyonEnv("PREPALERT_CANYON_"),
		canyon.WithServerAddress(opts.Address, opts.Prefix),
		canyon.WithWorkerBatchSize(opts.BatchSize),
	}

	switch strings.ToLower(opts.Mode) {
	case "http", "webhook":
		slog.InfoContext(ctx, "disable worker", "mode", opts.Mode)
		canyonOpts = append(canyonOpts, canyon.WithDisableWorker())
	case "worker":
		slog.InfoContext(ctx, "disable server", "mode", opts.Mode)
		canyonOpts = append(canyonOpts, canyon.WithDisableServer())
	default:
		// nothing to do
	}
	return canyon.RunWithContext(ctx, app.queueName, app, canyonOpts...)
}

func (app *App) Exec(ctx context.Context, alertID string) error {
	body, err := app.mkrSvc.NewEmulatedWebhookBody(ctx, alertID)
	if err != nil {
		return err
	}
	return app.ProcessRules(ctx, body)
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

type RequestIDGenerator interface {
	NextID() (uint64, error)
}

const (
	HeaderRequestID = "Prepalert-Request-ID"
)

var DefaultRequestIDGeneartor RequestIDGenerator = must(katsubushi.NewGenerator(1))

func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	ctx = slogutils.With(
		ctx,
		"x_amzn_trace_id", r.Header.Get("X-Amzn-Trace-Id"),
		"x_amz_cf_id", r.Header.Get("X-Amz-Cf-Id"),
	)
	if canyon.IsWorker(r) {
		if reqIDstr := r.Header.Get(HeaderRequestID); reqIDstr != "" {
			reqID, err := strconv.ParseUint(reqIDstr, 10, 64)
			if err != nil {
				canyon.Logger(r).WarnContext(
					ctx,
					"can not parse request id",
					"status", http.StatusBadRequest,
					"method", r.Method,
					"path", r.URL.Path,
				)
			} else {
				ctx = queryrunner.WithRequestID(ctx, fmt.Sprintf("%d", reqID))
				ctx = slogutils.With(ctx, "request_id", reqID)
			}
		}
		app.serveHTTPAsWorker(w, r.WithContext(ctx))
		return
	}
	reqID, err := DefaultRequestIDGeneartor.NextID()
	if err != nil {
		canyon.Logger(r).ErrorContext(
			ctx,
			"can not generate request id",
			"status", http.StatusInternalServerError,
			"method", r.Method,
			"path", r.URL.Path,
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set(HeaderRequestID, fmt.Sprintf("%d", reqID))
	r.Header.Set(HeaderRequestID, fmt.Sprintf("%d", reqID)) // set for worker
	ctx = slogutils.With(ctx, "request_id", reqID)
	app.serveHTTPAsWebhookServer(w, r.WithContext(ctx))
}

func (app *App) serveHTTPAsWebhookServer(w http.ResponseWriter, r *http.Request) {
	logger := canyon.Logger(r)
	ctx := r.Context()
	logger.InfoContext(ctx, "accept Webhook Server HTTP request", "method", r.Method, "path", r.URL.Path)
	if r.Method == http.MethodGet {
		app.backend.ServeHTTP(w, r)
		return
	}
	if app.EnableBasicAuth() && !app.CheckBasicAuth(r) {
		logger.InfoContext(ctx, "basic auth failed, request BasicAuth challenge", "status", http.StatusUnauthorized)
		w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		logger.InfoContext(ctx, "method not allowed", "status", http.StatusMethodNotAllowed)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	messageId, err := canyon.SendToWorker(r, nil)
	if err != nil {
		logger.InfoContext(ctx, "can not send to worker", "status", http.StatusInternalServerError, "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logger.InfoContext(ctx, "send to worker", "status", http.StatusOK, "sqs_message_id", messageId)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, http.StatusText(http.StatusOK))
}

func (app *App) serveHTTPAsWorker(w http.ResponseWriter, r *http.Request) {
	logger := canyon.Logger(r)
	ctx := r.Context()
	logger.InfoContext(
		ctx, "accept Worker request",
		"method", r.Method,
		"path", r.URL.Path,
		"sqs_message_id", r.Header.Get(canyon.HeaderSQSMessageId),
	)
	decoder := json.NewDecoder(r.Body)
	var body WebhookBody
	if err := decoder.Decode(&body); err != nil {
		logger.ErrorContext(ctx, "can not parse request body as Mackerel webhook body", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	ctx = slogutils.With(
		ctx,
		"alert_id", body.Alert.ID,
		"alsert_status", body.Alert.Status,
		"monitor", body.Alert.MonitorName,
	)
	logger.InfoContext(ctx, "parse request body as Mackerel webhook body")
	if err := app.ProcessRules(ctx, &body); err != nil {
		logger.ErrorContext(ctx, "failed process Mackerel webhook body", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logger.InfoContext(ctx, "finish process Mackerel webhook body")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, http.StatusText(http.StatusOK))
}

func (app *App) ProcessRules(ctx context.Context, body *WebhookBody) error {
	if body.Alert.IsOpen {
		slog.WarnContext(ctx, "alert is open, fill closed at now time")
		body.Alert.ClosedAt = flextime.Now().Unix()
	}
	slog.InfoContext(ctx, "start process rules")
	matchCount := 0
	for _, rule := range app.rules {
		if !rule.Match(body) {
			continue
		}
		slog.InfoContext(ctx, "match rule", "rule", rule.Name())
		matchCount++
		if err := rule.Render(ctx, app.evalCtx.NewChild(), body); err != nil {
			return fmt.Errorf("failed process Mackerel webhook body:%s: %w", rule.Name(), err)
		}
	}
	slog.InfoContext(ctx, "finish process rules", "matched_rule_count", matchCount)
	return nil
}

func (app *App) EnableBasicAuth() bool {
	return app.webhookClientID != "" && app.webhookClientSecret != ""
}

func (app *App) CheckBasicAuth(r *http.Request) bool {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return clientID == app.webhookClientID && clientSecret == app.webhookClientSecret
}
