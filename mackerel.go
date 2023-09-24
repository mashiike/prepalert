package prepalert

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Songmu/flextime"
	"github.com/mackerelio/mackerel-client-go"
)

//go:generate mockgen -source=$GOFILE -destination=./mock/mock_$GOFILE -package=mock

type MackerelClient interface {
	UpdateAlert(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error)
	FindGraphAnnotations(service string, from int64, to int64) ([]mackerel.GraphAnnotation, error)
	UpdateGraphAnnotation(annotationID string, annotation *mackerel.GraphAnnotation) (*mackerel.GraphAnnotation, error)
	CreateGraphAnnotation(annotation *mackerel.GraphAnnotation) (*mackerel.GraphAnnotation, error)
	GetOrg() (*mackerel.Org, error)
	GetAlert(string) (*mackerel.Alert, error)
	GetMonitor(string) (mackerel.Monitor, error)
	FindHost(id string) (*mackerel.Host, error)
}

type MackerelService struct {
	client          MackerelClient
	alertCacheMu    sync.Mutex
	alertCache      map[string]*mackerel.Alert
	alertCachedAt   map[string]time.Time
	monitorCacheMu  sync.Mutex
	monitorCache    map[string]mackerel.Monitor
	monitorCachedAt map[string]time.Time
}

func NewMackerelService(client MackerelClient) *MackerelService {
	return &MackerelService{
		client:          client,
		alertCache:      make(map[string]*mackerel.Alert),
		alertCachedAt:   make(map[string]time.Time),
		monitorCache:    make(map[string]mackerel.Monitor),
		monitorCachedAt: make(map[string]time.Time),
	}
}

var (
	GraphAnnotationDescriptionMaxSize = 1024
	AlertMemoMaxSize                  = 80 * 1000
	CacheDuration                     = 1 * time.Minute
)

func ExtructSection(memo string, header string) string {
	index := strings.Index(memo, header)
	if index == -1 {
		return ""
	}
	sectionText := strings.TrimPrefix(memo[index:], header)
	index = strings.Index(sectionText, "\n## ")
	if index == -1 {
		return header + sectionText
	}
	return header + sectionText[:index]
}

func (svc *MackerelService) newAlertMemo(ctx context.Context, alertID string, sectionName string, memo string) string {
	alert, err := svc.getAlertWithCache(ctx, alertID)
	if err != nil {
		slog.WarnContext(ctx, "get alert failed", "alert_id", alertID, "error", err)
		return memo
	}
	header := "## " + sectionName
	if alert.Memo == "" {
		slog.InfoContext(
			ctx,
			"alert memo is empty, add new section",
			"alert_id", alertID,
			"section_name", sectionName,
		)
		memo = triming(header+"\n"+memo, AlertMemoMaxSize, "...")
		return memo
	}
	extracted := ExtructSection(alert.Memo, header)
	if extracted == "" {
		slog.InfoContext(
			ctx,
			"alert memo section not found, add new section",
			"alert_id", alertID,
			"section_name", sectionName,
		)
		memo = header + "\n" + memo
		if len(alert.Memo)+len(memo) > AlertMemoMaxSize {
			memo = triming(memo, AlertMemoMaxSize-len(alert.Memo), "...")
			if !strings.HasPrefix(memo, header+"\n") {
				slog.WarnContext(ctx,
					"alert memo section not found, but memo is too long, so memo is truncated",
					"alert_id", alertID,
					"section_name", sectionName,
				)
				return alert.Memo
			}
		}
		return alert.Memo + "\n\n" + memo
	}
	slog.InfoContext(
		ctx,
		"alert memo section found, replace section",
		"alert_id", alertID,
		"section_name", sectionName,
	)
	memo = header + "\n" + memo + "\n"
	if len(alert.Memo)-len(extracted)+len(memo) > AlertMemoMaxSize {
		memo = triming(memo, AlertMemoMaxSize-len(alert.Memo)+len(extracted), "...\n")
	}
	memo = strings.Replace(alert.Memo, extracted, memo, 1)
	return memo
}

func (svc *MackerelService) UpdateAlertMemo(ctx context.Context, alertID string, sectionName string, memo string) error {
	svc.alertCacheMu.Lock()
	defer svc.alertCacheMu.Unlock()
	memo = svc.newAlertMemo(ctx, alertID, sectionName, memo)
	_, err := svc.client.UpdateAlert(alertID, mackerel.UpdateAlertParam{
		Memo: memo,
	})
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	slog.InfoContext(
		ctx,
		"updated alert memo",
		"alert_id", alertID,
	)
	if _, ok := svc.alertCache[alertID]; ok {
		svc.alertCache[alertID].Memo = memo
	}
	return nil
}

const (
	FindGraphAnnotationOffset = int64(15 * time.Minute / time.Second)
)

func (svc *MackerelService) PostGraphAnnotation(ctx context.Context, params *mackerel.GraphAnnotation) error {
	params.Description = triming(params.Description, GraphAnnotationDescriptionMaxSize, "...")
	annotations, err := svc.client.FindGraphAnnotations(params.Service, params.From-FindGraphAnnotationOffset, params.To+FindGraphAnnotationOffset)
	if err != nil {
		return fmt.Errorf("find graph annotations: %w", err)
	}
	for _, annotation := range annotations {
		slog.DebugContext(
			ctx,
			"check annotation",
			"annotation_id", annotation.ID,
			"annotation_title", annotation.Title,
		)
		if annotation.Title == params.Title {
			slog.InfoContext(
				ctx,
				"annotation is aleady exists, overwrite description",
				"annotation_id", annotation.ID,
			)
			annotation.Description = params.Description
			annotation.Service = params.Service
			_, err := svc.client.UpdateGraphAnnotation(annotation.ID, &annotation)
			if err != nil {
				return fmt.Errorf("update graph annotations: %w", err)
			}
			return nil
		}
	}
	slog.InfoContext(
		ctx,
		"create new annotation",
	)
	output, err := svc.client.CreateGraphAnnotation(params)
	if err != nil {
		return fmt.Errorf("create graph annotations: %w", err)
	}
	slog.InfoContext(
		ctx,
		"annotation created",
		"annotation_id", output.ID,
	)
	return nil
}

type WebhookBody struct {
	OrgName  string   `json:"orgName" cty:"org_name"`
	Text     string   `json:"text" cty:"-"`
	Event    string   `json:"event" cty:"event"`
	ImageURL *string  `json:"imageUrl" cty:"image_url"`
	Memo     string   `json:"memo" cty:"memo"`
	Host     *Host    `json:"host,omitempty" cty:"host,omitempty"`
	Service  *Service `json:"service,omitempty" cty:"service,omitempty"`
	Alert    *Alert   `json:"alert" cty:"alert,omitempty"`
}

//go:embed example_webhook.json
var exampleWebhookJSON []byte

func (svc *MackerelService) NewExampleWebhookBody() *WebhookBody {
	var body WebhookBody
	if err := json.Unmarshal(exampleWebhookJSON, &body); err != nil {
		panic(err)
	}
	return &body
}

func (svc *MackerelService) GetAlertWithCache(ctx context.Context, alertID string) (*mackerel.Alert, error) {
	svc.alertCacheMu.Lock()
	defer svc.alertCacheMu.Unlock()
	return svc.getAlertWithCache(ctx, alertID)
}

func (svc *MackerelService) getAlertWithCache(ctx context.Context, alertID string) (*mackerel.Alert, error) {
	if cachedAt, ok := svc.alertCachedAt[alertID]; ok && time.Since(cachedAt) < CacheDuration {
		return svc.alertCache[alertID], nil
	}
	alert, err := svc.client.GetAlert(alertID)
	if err != nil {
		return nil, fmt.Errorf("get alert:%w", err)
	}
	svc.alertCache[alertID] = alert
	svc.alertCachedAt[alertID] = flextime.Now()
	return alert, nil
}

func (svc *MackerelService) GetMonitorWithCache(ctx context.Context, monitorID string) (mackerel.Monitor, error) {
	svc.monitorCacheMu.Lock()
	defer svc.monitorCacheMu.Unlock()
	return svc.getMonitorWithCache(ctx, monitorID)
}

func (svc *MackerelService) getMonitorWithCache(ctx context.Context, monitorID string) (mackerel.Monitor, error) {
	if cachedAt, ok := svc.monitorCachedAt[monitorID]; ok && time.Since(cachedAt) < CacheDuration {
		return svc.monitorCache[monitorID], nil
	}
	monitor, err := svc.client.GetMonitor(monitorID)
	if err != nil {
		return nil, fmt.Errorf("get monitor:%w", err)
	}
	svc.monitorCache[monitorID] = monitor
	svc.monitorCachedAt[monitorID] = flextime.Now()
	return monitor, nil
}

func (svc *MackerelService) GetMonitorByAlertID(ctx context.Context, alertID string) (mackerel.Monitor, error) {
	alert, err := svc.GetAlertWithCache(ctx, alertID)
	if err != nil {
		return nil, fmt.Errorf("get alert:%w", err)
	}
	monitor, err := svc.client.GetMonitor(alert.MonitorID)
	if err != nil {
		return nil, fmt.Errorf("get monitor:%w", err)
	}
	return monitor, nil
}

func (svc *MackerelService) NewEmulatedWebhookBody(ctx context.Context, alertID string) (*WebhookBody, error) {
	org, err := svc.client.GetOrg()
	if err != nil {
		return nil, fmt.Errorf("get org:%w", err)
	}
	alert, err := svc.GetAlertWithCache(ctx, alertID)
	if err != nil {
		return nil, fmt.Errorf("get alert:%w", err)
	}
	monitor, err := svc.getMonitorWithCache(ctx, alert.MonitorID)
	if err != nil {
		return nil, fmt.Errorf("get monitor:%w", err)
	}
	body := &WebhookBody{
		OrgName: org.Name,
		Event:   "alert",
		Alert: &Alert{
			OpenedAt:        alert.OpenedAt,
			CreatedAt:       alert.OpenedAt * 1000,
			Duration:        0,
			IsOpen:          !strings.EqualFold(alert.Status, "ok"),
			MetricLabel:     "",
			MetricValue:     alert.Value,
			MonitorName:     monitor.MonitorName(),
			MonitorOperator: "",
			Status:          strings.ToLower(alert.Status),
			Trigger:         "monitor",
			ID:              alert.ID,
			URL:             fmt.Sprintf("https://mackerel.io/orgs/%s/alerts/%s", org.Name, alert.ID),
		},
	}
	if alert.ClosedAt != 0 {
		body.Alert.ClosedAt = &alert.ClosedAt
		body.Alert.Duration = alert.ClosedAt - alert.OpenedAt
	}
	switch m := monitor.(type) {
	case *mackerel.MonitorConnectivity:
		body.Memo = m.Memo
	case *mackerel.MonitorHostMetric:
		body.Memo = m.Memo
		body.Alert.WarningThreshold = m.Warning
		body.Alert.CriticalThreshold = m.Critical
		body.Alert.MonitorOperator = m.Operator
		body.Alert.Duration = int64(m.Duration)
		body.Alert.MetricLabel = m.Metric
		host, err := svc.client.FindHost(alert.HostID)
		if err != nil {
			return nil, fmt.Errorf("find host:%w", err)
		}
		body.Host = &Host{
			ID:        host.ID,
			Name:      host.Name,
			Memo:      host.Memo,
			URL:       fmt.Sprintf("https://mackerel.io/orgs/%s/hosts/%s", org.Name, host.ID),
			Status:    host.Status,
			IsRetired: host.IsRetired,
		}
		for serviceName, roleNames := range host.Roles {
			for _, roleName := range roleNames {
				body.Host.Roles = append(body.Host.Roles, &Role{
					Fullname:    fmt.Sprintf("%s: %s", serviceName, roleName),
					ServiceName: serviceName,
					RoleName:    roleName,
					ServiceURL:  fmt.Sprintf("https://mackerel.io/orgs/%s/services/%s", org.Name, serviceName),
					RoleURL:     fmt.Sprintf("https://mackerel.io/orgs/%s/services/%s#role=%s", org.Name, serviceName, roleName),
				})
			}
		}
	case *mackerel.MonitorServiceMetric:
		body.Memo = m.Memo
		body.Alert.WarningThreshold = m.Warning
		body.Alert.CriticalThreshold = m.Critical
		body.Alert.MonitorOperator = m.Operator
		body.Alert.Duration = int64(m.Duration)
		body.Alert.MetricLabel = m.Metric
		body.Service = &Service{
			Name:  m.Service,
			OrgID: org.Name,
		}
	case *mackerel.MonitorExternalHTTP:
		body.Memo = m.Memo
		body.Service = &Service{
			Name:  m.Service,
			OrgID: org.Name,
		}
	case *mackerel.MonitorExpression:
		body.Memo = m.Memo
		body.Alert.WarningThreshold = m.Warning
		body.Alert.CriticalThreshold = m.Critical
		body.Alert.MonitorOperator = m.Operator
	case *mackerel.MonitorAnomalyDetection:
		body.Memo = m.Memo
	default:
		return nil, fmt.Errorf("unknown monitor type: %s", m.MonitorName())
	}

	return body, nil
}

type Host struct {
	ID        string  `json:"id" cty:"id"`
	Name      string  `json:"name" cty:"name"`
	URL       string  `json:"url" cty:"url"`
	Type      string  `json:"type,omitempty" cty:"type"`
	Status    string  `json:"status" cty:"status"`
	Memo      string  `json:"memo" cty:"memo"`
	IsRetired bool    `json:"isRetired" cty:"is_retired"`
	Roles     []*Role `json:"roles" cty:"roles,omitempty"`
}

type Role struct {
	Fullname    string `json:"fullname" cty:"fullname"`
	ServiceName string `json:"serviceName" cty:"service_name"`
	ServiceURL  string `json:"serviceUrl" cty:"service_url"`
	RoleName    string `json:"roleName" cty:"role_name"`
	RoleURL     string `json:"roleUrl" cty:"role_url"`
}

type Service struct {
	ID    string  `json:"id" cty:"id"`
	Memo  string  `json:"memo" cty:"memo"`
	Name  string  `json:"name" cty:"name"`
	OrgID string  `json:"orgId" cty:"org_id"`
	Roles []*Role `json:"roles" cty:"roles,omitempty"`
}

type Alert struct {
	OpenedAt          int64    `json:"openedAt" cty:"opened_at"`
	ClosedAt          *int64   `json:"closedAt" cty:"closed_at"`
	CreatedAt         int64    `json:"createdAt" cty:"created_at"`
	CriticalThreshold *float64 `json:"criticalThreshold,omitempty" cty:"critical_threshold,omitempty"`
	Duration          int64    `json:"duration" cty:"duration"`
	IsOpen            bool     `json:"isOpen" cty:"is_open"`
	MetricLabel       string   `json:"metricLabel" cty:"metric_label"`
	MetricValue       float64  `json:"metricValue" cty:"metric_value"`
	MonitorName       string   `json:"monitorName" cty:"monitor_name"`
	MonitorOperator   string   `json:"monitorOperator" cty:"monitor_operator"`
	Status            string   `json:"status" cty:"status"`
	Trigger           string   `json:"trigger" cty:"trigger"`
	ID                string   `json:"id" cty:"id"`
	URL               string   `json:"url" cty:"url"`
	WarningThreshold  *float64 `json:"warningThreshold,omitempty" cty:"warning_threshold,omitempty"`
}

func (svc *MackerelService) GetMonitorName(ctx context.Context, monitorID string) (string, error) {
	slog.DebugContext(ctx, "call MackerelService.GetMonitorName", "monitor_id", monitorID)
	monitor, err := svc.getMonitorWithCache(ctx, monitorID)
	if err != nil {
		return "", fmt.Errorf("get monitor:%w", err)
	}
	return monitor.MonitorName(), nil
}
