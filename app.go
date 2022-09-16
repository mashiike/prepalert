package prepalert

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/fujiwara/ridge"
	"github.com/kayac/go-katsubushi"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/grat"
	"github.com/mashiike/ls3viewer"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/mashiike/prepalert/queryrunner"
)

type App struct {
	client    *mackerel.Client
	auth      *hclconfig.AuthBlock
	backend   *hclconfig.S3BackendBlock
	rules     []*Rule
	service   string
	queueUrl  string
	sqsClient *sqs.Client
	uploader  *manager.Uploader
	viewer    http.Handler
}

func New(apikey string, cfg *hclconfig.Config) (*App, error) {
	client := mackerel.NewClient(apikey)
	rules := make([]*Rule, 0, len(cfg.Rules))
	for i, ruleBlock := range cfg.Rules {
		rule, err := NewRule(client, ruleBlock)
		if err != nil {
			return nil, fmt.Errorf("rules[%d]:%w", i, err)
		}
		rules = append(rules, rule)
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load aws default config:%w", err)
	}
	sqsClient := sqs.NewFromConfig(awsCfg)
	log.Printf("[info] try get sqs queue url: %s", cfg.Prepalert.SQSQueueName)
	output, err := sqsClient.GetQueueUrl(context.Background(), &sqs.GetQueueUrlInput{
		QueueName: aws.String(cfg.Prepalert.SQSQueueName),
	})
	if err != nil {
		return nil, fmt.Errorf("can not get sqs queu url:%w", err)
	}
	app := &App{
		client:    client,
		auth:      cfg.Prepalert.Auth,
		rules:     rules,
		service:   cfg.Prepalert.Service,
		sqsClient: sqsClient,
		queueUrl:  *output.QueueUrl,
	}
	if backend := cfg.Prepalert.S3Backend; !backend.IsEmpty() {
		log.Printf("[info] enable s3 backend: s3://%s", backend.BucketName)
		app.backend = backend
		s3Client := s3.NewFromConfig(awsCfg)
		app.uploader = manager.NewUploader(s3Client)
		viewerOptFns := []func(*ls3viewer.Options){
			ls3viewer.WithBaseURL(backend.ViewerBaseURL.String()),
		}
		if app.EnableBasicAuth() && !backend.EnableGoogleAuth() {
			viewerOptFns = append(viewerOptFns, ls3viewer.WithBasicAuth(app.auth.ClientID, app.auth.ClientSecret))
		}
		if backend.EnableGoogleAuth() {
			viewerOptFns = append(viewerOptFns, ls3viewer.WithGoogleOIDC(
				*backend.ViewerGoogleClientID,
				*backend.ViewerGoogleClientSecret,
				backend.ViewerSessionEncryptKey,
				backend.Allowed,
				backend.Denied,
			))
		}
		h, err := ls3viewer.New(backend.BucketName, *backend.ObjectKeyPrefix, viewerOptFns...)
		if err != nil {
			return nil, fmt.Errorf("initialize ls3viewer:%w", err)
		}
		app.viewer = h
	}
	return app, nil
}

type RunOptions struct {
	Mode      string
	Address   string
	Prefix    string
	BatchSize int
}

func (app *App) Run(ctx context.Context, opts RunOptions) error {
	switch strings.ToLower(opts.Mode) {
	case "webhook", "http":
		if strings.EqualFold(opts.Mode, "webhook") {
			log.Println("[warn] mode webhook is deprecated. change to http")
		}
		log.Println("[info] Run as http")
		if app.EnableBasicAuth() {
			log.Printf("[info] with basic auth: client_id=%s", app.auth.ClientID)
		}
		ridge.RunWithContext(ctx, opts.Address, opts.Prefix, app)
	case "worker":
		log.Println("[info] Run as worker")
		return grat.RunWithContext(ctx, app.queueUrl, opts.BatchSize, app.HandleSQS)
	}
	return nil
}

func (app *App) Exec(ctx context.Context, alertID string) error {
	body, err := app.NewWebhookBody(ctx, alertID)
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
	reqID, err := generator.NextID()
	if err != nil {
		log.Println("[info]", "-", r.Method, r.URL.Path)
		log.Println("[error] can not get reqID")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-Request-ID", fmt.Sprintf("%d", reqID))
	log.Println("[info]", reqID, r.Method, r.URL.Path)
	if r.Method == http.MethodGet {
		if !app.EnableBackend() {
			log.Printf("[info][%d] status=%d", reqID, http.StatusMethodNotAllowed)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		app.viewer.ServeHTTP(w, r)
		return
	}
	if app.EnableBasicAuth() && !app.CheckBasicAuth(r) {
		log.Printf("[info][%d] status=%d", reqID, http.StatusUnauthorized)
		w.Header().Add("WWW-Authenticate", `Basic realm="SECRET AREA"`)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		log.Printf("[info][%d] status=%d", reqID, http.StatusMethodNotAllowed)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[info][%d] can not read body:%v", reqID, err)
		log.Printf("[info][%d] status=%d", reqID, http.StatusBadRequest)
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
		log.Printf("[info][%d] can send sqs message:%v", reqID, err)
		log.Printf("[info][%d] status=%d", reqID, http.StatusInternalServerError)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log.Printf("[info][%d] send sqs message: message id is %s", reqID, *output.MessageId)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, http.StatusText(http.StatusOK))
}

func (app *App) HandleSQS(ctx context.Context, event *events.SQSEvent) (*grat.BatchItemFailureResponse, error) {
	resp := &grat.BatchItemFailureResponse{}
	for _, message := range event.Records {
		if err := app.handleSQSMessage(ctx, &message); err != nil {
			if len(event.Records) == 1 {
				return nil, err
			}
			log.Printf("[warn] handle message failed, messageID=%s, reqID=%d", message.MessageId, getRequestIDFromSQSMessage(&message))
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
					log.Printf("[warn] message attribute parse faield id=%s :%v", message.MessageId, err)
				}
				return reqID
			}
		}
	}
	return 0
}
func (app *App) handleSQSMessage(ctx context.Context, message *events.SQSMessage) error {
	reqID := getRequestIDFromSQSMessage(message)
	log.Printf("[info][%d] handle message id=%s", reqID, message.MessageId)
	decoder := json.NewDecoder(strings.NewReader(message.Body))
	var body WebhookBody
	if err := decoder.Decode(&body); err != nil {
		log.Printf("[error][%d] sqs message can not parse as Mackerel webhook body: %v", reqID, err)
		return err
	}
	log.Printf("[info][%d] hendle webhook id=%s, status=%s monitor=%s", reqID, body.Alert.ID, body.Alert.Status, body.Alert.MonitorName)
	if body.Alert.IsOpen {
		log.Printf("[info][%d] alert is not closed, skip nothing todo id=%s, status=%s monitor=%s", reqID, body.Alert.ID, body.Alert.Status, body.Alert.MonitorName)
		return nil
	}
	ctx = app.WithQueryRunningContext(ctx, reqID, message)
	return app.ProcessRules(ctx, &body)
}
func (app *App) ProcessRules(ctx context.Context, body *WebhookBody) error {
	matchCount := 0
	for _, rule := range app.rules {
		if !rule.Match(body) {
			continue
		}
		log.Printf("[info] match rule `%s`", rule.Name())
		matchCount++
		if err := app.ProcessRule(ctx, rule, body); err != nil {
			return fmt.Errorf("failed process Mackerel webhook body:%s: %w", rule.Name(), err)
		}
	}
	if matchCount == 0 {
		log.Printf("[info] no match rules")
	}
	return nil
}

func (app *App) EnableBasicAuth() bool {
	return !app.auth.IsEmpty()
}

func (app *App) EnableBackend() bool {
	return !app.backend.IsEmpty()
}

func (app *App) CheckBasicAuth(r *http.Request) bool {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return clientID == app.auth.ClientID && clientSecret == app.auth.ClientSecret
}

func (app *App) WithQueryRunningContext(ctx context.Context, reqID uint64, message *events.SQSMessage) context.Context {
	hctx := queryrunner.NewQueryRunningContext(app.sqsClient, app.queueUrl, reqID, message)
	return queryrunner.WithQueryRunningContext(ctx, hctx)
}
