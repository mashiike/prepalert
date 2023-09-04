package prepalert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/fujiwara/ridge"
	"github.com/hashicorp/hcl/v2"
	"github.com/kayac/go-katsubushi"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/grat"
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
	queueUrl            string
	sqsClient           *sqs.Client
	evalCtx             *hcl.EvalContext
}

func New(apikey string, cfg *hclconfig.Config) (*App, error) {
	client := mackerel.NewClient(apikey)
	svc := NewMackerelService(client)
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load aws default config:%w", err)
	}

	var backend Backend
	switch {
	case !cfg.Prepalert.S3Backend.IsEmpty():
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

	sqsClient := sqs.NewFromConfig(awsCfg)
	slog.Info("try get sqs queue url", "sqs_queue_name", cfg.Prepalert.SQSQueueName)
	output, err := sqsClient.GetQueueUrl(context.Background(), &sqs.GetQueueUrlInput{
		QueueName: aws.String(cfg.Prepalert.SQSQueueName),
	})
	if err != nil {
		return nil, fmt.Errorf("can not get sqs queu url:%w", err)
	}
	app := &App{
		mkrSvc:              svc,
		backend:             backend,
		webhookClientID:     cfg.Prepalert.Auth.ClientID,
		webhookClientSecret: cfg.Prepalert.Auth.ClientSecret,
		rules:               rules,
		sqsClient:           sqsClient,
		queueUrl:            *output.QueueUrl,
		evalCtx:             cfg.EvalContext,
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
	switch strings.ToLower(opts.Mode) {
	case "webhook", "http":
		if strings.EqualFold(opts.Mode, "webhook") {
			slog.WarnContext(ctx, "mode webhook is deprecated. change to http")
		}
		slog.InfoContext(ctx, "run as http", "address", opts.Address, "prefix", opts.Prefix)
		ridge.RunWithContext(ctx, opts.Address, opts.Prefix, app)
	case "worker":
		slog.InfoContext(ctx, "run as worker", "batch_size", opts.BatchSize)
		return grat.RunWithContext(ctx, app.queueUrl, opts.BatchSize, app.HandleSQS)
	}
	return nil
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

var generator = must(katsubushi.NewGenerator(1))

const requestIDAttributeKey = "RequestID"

func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	ctx = slogutils.With(
		ctx,
		"x_amzn_trace_id", r.Header.Get("X-Amzn-Trace-Id"),
		"x_amz_cf_id", r.Header.Get("X-Amz-Cf-Id"),
	)
	reqID, err := generator.NextID()
	if err != nil {
		slog.ErrorContext(
			ctx,
			"can not generate request id",
			"status", http.StatusInternalServerError,
			"method", r.Method,
			"path", r.URL.Path,
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-Request-ID", fmt.Sprintf("%d", reqID))
	ctx = slogutils.With(ctx, "request_id", reqID)
	r = r.WithContext(ctx)
	slog.InfoContext(ctx, "accept HTTP request", "method", r.Method, "path", r.URL.Path)
	if r.Method == http.MethodGet {
		app.backend.ServeHTTP(w, r)
		return
	}
	if app.EnableBasicAuth() && !app.CheckBasicAuth(r) {
		slog.InfoContext(ctx, "basic auth failed, request BasicAuth challenge", "status", http.StatusUnauthorized)
		w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		slog.InfoContext(ctx, "method not allowed", "status", http.StatusMethodNotAllowed)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		slog.InfoContext(ctx, "can not read body", "status", http.StatusBadRequest, "error", err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	output, err := app.sqsClient.SendMessage(r.Context(), &sqs.SendMessageInput{
		MessageBody: aws.String(string(bs)),
		QueueUrl:    aws.String(app.queueUrl),
		MessageAttributes: map[string]types.MessageAttributeValue{
			requestIDAttributeKey: {
				DataType:    aws.String("Number"),
				StringValue: aws.String(fmt.Sprintf("%d", reqID)),
			},
		},
	})
	if err != nil {
		slog.InfoContext(ctx, "can not send sqs message", "status", http.StatusInternalServerError, "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	slog.InfoContext(ctx, "send sqs message", "status", http.StatusOK, "message_id", *output.MessageId)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, http.StatusText(http.StatusOK))
}

func (app *App) HandleSQS(ctx context.Context, event *events.SQSEvent) (*grat.BatchItemFailureResponse, error) {
	resp := &grat.BatchItemFailureResponse{}
	for _, message := range event.Records {
		ctxWithMetadata := slogutils.With(
			ctx,
			"message_id", message.MessageId,
			"request_id", getRequestIDFromSQSMessage(&message),
		)
		if err := app.handleSQSMessage(ctxWithMetadata, &message); err != nil {
			if len(event.Records) == 1 {
				return nil, err
			}
			slog.WarnContext(ctxWithMetadata, "handle message failed", "error", err.Error())
			resp.BatchItemFailures = append(resp.BatchItemFailures, grat.BatchItemFailureItem{
				ItemIdentifier: message.MessageId,
			})
		}
	}
	return resp, nil
}

func getRequestIDFromSQSMessage(message *events.SQSMessage) uint64 {
	if message.MessageAttributes != nil {
		if attr, ok := message.MessageAttributes[requestIDAttributeKey]; ok {
			if strings.EqualFold(attr.DataType, "number") && attr.StringValue != nil {
				reqID, err := strconv.ParseUint(*attr.StringValue, 10, 64)
				if err != nil {
					slog.Warn("message attribute parse faield", "id", message.MessageId, "error", err.Error())
				}
				return reqID
			}
		}
	}
	return 0
}

func (app *App) handleSQSMessage(ctx context.Context, message *events.SQSMessage) error {
	slog.InfoContext(ctx, "handle sqs message")
	decoder := json.NewDecoder(strings.NewReader(message.Body))
	var body WebhookBody
	if err := decoder.Decode(&body); err != nil {
		slog.ErrorContext(ctx, "can not parse sqs message as Mackerel webhook body", "error", err.Error())
		return err
	}
	ctx = slogutils.With(
		ctx,
		"alert_id", body.Alert.ID,
		"alsert_status", body.Alert.Status,
		"monitor", body.Alert.MonitorName,
	)
	slog.InfoContext(ctx, "parse sqs message as Mackerel webhook body")
	ctx = app.WithQueryRunningContext(ctx, message)
	return app.ProcessRules(ctx, &body)
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

func (app *App) WithQueryRunningContext(ctx context.Context, message *events.SQSMessage) context.Context {
	reqID := getRequestIDFromSQSMessage(message)
	ctx = queryrunner.WithRequestID(ctx, fmt.Sprintf("%d", reqID))
	ctx = queryrunner.WithTimeoutExtender(ctx, queryrunner.TimeoutExtenderFunc(
		func(ctx context.Context, timeout time.Duration) error {
			_, err := app.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
				QueueUrl:          aws.String(app.queueUrl),
				ReceiptHandle:     aws.String(message.ReceiptHandle),
				VisibilityTimeout: int32(timeout.Seconds()),
			})
			if err != nil {
				return err
			}
			return nil
		},
	))
	return ctx
}
