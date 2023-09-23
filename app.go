package prepalert

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/kayac/go-katsubushi"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/canyon"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert/provider"
	"github.com/mashiike/slogutils"
)

type App struct {
	mkrSvc                *MackerelService
	backend               Backend
	rules                 []*Rule
	queueName             string
	webhookClientID       string
	webhookClientSecret   string
	providerParameters    provider.ProviderParameters
	providers             map[string]provider.Provider
	queries               map[string]provider.Query
	diagWriter            *hclutil.DiagnosticsWriter
	evalCtx               *hcl.EvalContext
	loadingConfig         bool
	workerPrepared        bool
	webhookServerPrepared bool
	cleanupFuncs          []func() error
}

func New(apikey string) *App {
	app := &App{
		backend: NewDiscardBackend(),
	}
	return app.SetMackerelClient(mackerel.NewClient(apikey))
}

func (app *App) Close() error {
	var errs []error
	if c, ok := app.backend.(io.Closer); ok {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, provider := range app.providers {
		if c, ok := provider.(io.Closer); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for _, query := range app.queries {
		if c, ok := query.(io.Closer); ok {
			if err := c.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for _, cleanup := range app.cleanupFuncs {
		if err := cleanup(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (app *App) WorkerIsReady() bool {
	return app.workerPrepared
}

func (app *App) WebhookServerIsReady() bool {
	return app.webhookServerPrepared
}

func (app *App) EnableWebhookServer() bool {
	return app.SQSQueueName() != ""
}

func (app *App) SetMackerelClient(client MackerelClient) *App {
	app.mkrSvc = NewMackerelService(client)
	return app
}

func (app *App) SQSQueueName() string {
	return app.queueName
}

func (app *App) MackerelService() *MackerelService {
	return app.mkrSvc
}

func (app *App) Rules() []*Rule {
	return app.rules
}

func (app *App) Backend() Backend {
	return app.backend
}

func (app *App) ProviderList() []string {
	providers := make([]string, 0, len(app.providers))
	for name := range app.providers {
		providers = append(providers, name)
	}
	return providers
}

func (app *App) QueryList() []string {
	queries := make([]string, 0, len(app.queries))
	for name := range app.queries {
		queries = append(queries, name)
	}
	return queries
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
	if body.Alert == nil {
		logger.WarnContext(ctx, "not found alert in request body, maybe not webhook request", "text", body.Text, "org_name", body.OrgName, "event", body.Event)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, http.StatusText(http.StatusOK))
		return
	}
	ctx = slogutils.With(
		ctx,
		"alert_id", body.Alert.ID,
		"alsert_status", body.Alert.Status,
		"monitor", body.Alert.MonitorName,
	)
	logger.InfoContext(ctx, "parse request body as Mackerel webhook body")
	if err := app.ExecuteRules(ctx, &body); err != nil {
		logger.ErrorContext(ctx, "failed process Mackerel webhook body", "error", err.Error())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logger.InfoContext(ctx, "finish process Mackerel webhook body")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, http.StatusText(http.StatusOK))
}

func (app *App) ExecuteRules(ctx context.Context, body *WebhookBody) error {
	slog.InfoContext(ctx, "start process rules")
	matchCount := 0
	evalCtx, err := app.NewEvalContext(body)
	if err != nil {
		return fmt.Errorf("failed build eval context: %w", err)
	}
	for _, rule := range app.rules {
		if !rule.Match(evalCtx) {
			continue
		}
		slog.InfoContext(ctx, "match rule", "rule", rule.Name())
		matchCount++
		if err := app.ExecuteRule(ctx, evalCtx, rule, body); err != nil {
			return fmt.Errorf("failed process Mackerel webhook body:%s: %w", rule.Name(), err)
		}
	}
	slog.InfoContext(ctx, "finish process rules", "matched_rule_count", matchCount)
	return nil
}

func (app *App) ExecuteRule(ctx context.Context, evalCtx *hcl.EvalContext, rule *Rule, body *WebhookBody) error {
	ctx = slogutils.With(ctx, "rule_name", rule.Name())
	dependsOn := rule.DependsOnQueries()
	var mu sync.Mutex
	var wg sync.WaitGroup
	var errs []error
	for _, queryFQN := range dependsOn {
		query, ok := app.queries[queryFQN]
		if !ok {
			errs = append(errs, fmt.Errorf("query not found %q on %s", queryFQN, rule.Name()))
			continue
		}
		mu.Lock()
		evalCtxQueryVariables := &provider.EvalContextQueryVariables{
			FQN:    queryFQN,
			Status: "running",
		}
		var err error
		evalCtx, err = provider.WithQury(evalCtx, evalCtxQueryVariables)
		mu.Unlock()
		if err != nil {
			errs = append(errs, fmt.Errorf("failed set query status %q on %s: %w", queryFQN, rule.Name(), err))
			continue
		}
		wg.Add(1)
		go func(v *provider.EvalContextQueryVariables, query provider.Query) {
			defer func() {
				wg.Done()
			}()
			egctxWithQueryName := slogutils.With(
				ctx,
				"query", v.FQN,
			)
			slog.InfoContext(egctxWithQueryName, "start run query")
			result, err := query.Run(egctxWithQueryName, evalCtx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				var diags hcl.Diagnostics
				slog.DebugContext(
					egctxWithQueryName,
					"failed run query",
					"error", err.Error(),
					"errType", fmt.Sprintf("%T", err),
				)
				if errors.As(err, &diags) {
					app.diagWriter.WriteDiagnostics(diags)
				}
				errs = append(errs, err)
				slog.WarnContext(egctxWithQueryName, "failed run query", "reason", err.Error())
				v.Status = "failed"
				v.Error = err.Error()
				evalCtx, err = provider.WithQury(evalCtx, v)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed set query status %q on %s: %w", v.FQN, rule.Name(), err))
				}
				return
			}
			v.Status = "success"
			v.Result = result
			evalCtx, err = provider.WithQury(evalCtx, v)
			if err != nil {
				slog.ErrorContext(egctxWithQueryName, "failed marshal query result", "error", err.Error())
				err = app.UnwrapAndDumpDiagnoctics(err)
				errs = append(errs, fmt.Errorf("failed set query status %q on %s: %w", v.FQN, rule.Name(), err))
			}
			if err := rule.Execute(ctx, evalCtx); err != nil {
				err = app.UnwrapAndDumpDiagnoctics(err)
				slog.ErrorContext(egctxWithQueryName, "failed execute rule", "error", err.Error())
				errs = append(errs, err)
				return
			}
			slog.InfoContext(egctxWithQueryName, "end run query")
		}(evalCtxQueryVariables, query)
	}
	wg.Wait()
	if len(errs) > 0 {
		return fmt.Errorf("failed process rule %q: %w", rule.Name(), errors.New("query failed"))
	}
	if len(dependsOn) > 0 {
		return nil
	}
	return rule.Execute(ctx, evalCtx)
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

func (app *App) UnwrapAndDumpDiagnoctics(err error) error {
	var diags hcl.Diagnostics
	if errors.As(err, &diags) {
		app.diagWriter.WriteDiagnostics(diags)
		return err
	}
	return err
}
