package prepalert

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/fujiwara/ridge"
	"github.com/kayac/go-katsubushi"
	"github.com/mackerelio/mackerel-client-go"
)

type App struct {
	client  *mackerel.Client
	auth    *AuthConfig
	rules   []*Rule
	service string
}

func New(apikey string, cfg *Config) (*App, error) {
	client := mackerel.NewClient(apikey)
	queryRunners, err := NewQueryRunners(cfg.QueryRunners)
	if err != nil {
		return nil, fmt.Errorf("build query runners:%w", err)
	}
	rules := make([]*Rule, 0, len(cfg.Rules))
	for i, ruleCfg := range cfg.Rules {
		rule, err := NewRule(client, ruleCfg, queryRunners)
		if err != nil {
			return nil, fmt.Errorf("rules[%d]:%w", i, err)
		}
		rules = append(rules, rule)
	}
	app := &App{
		client:  client,
		auth:    cfg.Auth,
		rules:   rules,
		service: cfg.Service,
	}
	return app, nil
}

func (app *App) Run(address string, prefix string) {
	app.RunWithContext(context.Background(), address, prefix)
}

func (app *App) RunWithContext(ctx context.Context, address string, prefix string) {
	if app.EnableBasicAuth() {
		log.Printf("[info] with basic auth: client_id=%s", app.auth.ClientID)
	}
	ridge.RunWithContext(ctx, address, prefix, app)
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

var generator = must(katsubushi.NewGenerator(1))

func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	reqID, err := generator.NextID()
	if err != nil {
		log.Println("[error] can not get reqID")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-Request-ID", fmt.Sprintf("%d", reqID))
	log.Printf("[info][%d] %s %s %s", reqID, r.Proto, r.Method, r.URL)
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
	decoder := json.NewDecoder(r.Body)
	var body WebhookBody
	if err := decoder.Decode(&body); err != nil {
		log.Printf("[error][%d] decode body=%v, status=%d", reqID, err, http.StatusBadRequest)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	log.Printf("[info][%d] hendle webhook id=%s, status=%s monitor=%s", reqID, body.Alert.ID, body.Alert.Status, body.Alert.MonitorName)
	if body.Alert.IsOpen {
		log.Printf("[info][%d] alert is not closed, skip nothing todo id=%s, status=%s monitor=%s", reqID, body.Alert.ID, body.Alert.Status, body.Alert.MonitorName)
		return
	}
	app.HandleWebhook(w, r, reqID, &body)
}

func (app *App) EnableBasicAuth() bool {
	return app.auth != nil
}

func (app *App) CheckBasicAuth(r *http.Request) bool {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return clientID == app.auth.ClientID && clientSecret == app.auth.ClientSecret
}
