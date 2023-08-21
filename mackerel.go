package prepalert

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/zclconf/go-cty/cty"
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
	client MackerelClient
}

func NewMackerelService(client MackerelClient) *MackerelService {
	return &MackerelService{
		client: client,
	}
}

func (svc *MackerelService) UpdateAlertMemo(ctx context.Context, alertID string, memo string) error {
	slog.InfoContext(
		ctx,
		"update alert memo",
		"alert_id", alertID,
	)
	_, err := svc.client.UpdateAlert(alertID, mackerel.UpdateAlertParam{
		Memo: memo,
	})
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	return nil
}

const (
	FindGraphAnnotationOffset = int64(15 * time.Minute / time.Second)
)

func (svc *MackerelService) PostGraphAnnotation(ctx context.Context, params *mackerel.GraphAnnotation) error {
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
	OrgName  string   `json:"orgName"`
	Event    string   `json:"event"`
	ImageURL *string  `json:"imageUrl"`
	Memo     string   `json:"memo"`
	Host     *Host    `json:"host,omitempty"`
	Service  *Service `json:"service,omitempty"`
	Alert    *Alert   `json:"alert"`
}

func (svc *MackerelService) NewEmulatedWebhookBody(ctx context.Context, alertID string) (*WebhookBody, error) {
	org, err := svc.client.GetOrg()
	if err != nil {
		return nil, fmt.Errorf("get org:%w", err)
	}
	alert, err := svc.client.GetAlert(alertID)
	if err != nil {
		return nil, fmt.Errorf("get alert:%w", err)
	}
	monitor, err := svc.client.GetMonitor(alert.MonitorID)
	if err != nil {
		return nil, fmt.Errorf("get monitor:%w", err)
	}
	body := &WebhookBody{
		OrgName: org.Name,
		Event:   "alert",
		Alert: &Alert{
			OpenedAt:        alert.OpenedAt,
			ClosedAt:        alert.ClosedAt,
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

func (body *WebhookBody) MarshalCTYValues() map[string]cty.Value {
	values := map[string]cty.Value{
		"org_name":  cty.StringVal(body.OrgName),
		"event":     cty.StringVal(body.Event),
		"image_url": cty.StringVal(body.Event),
		"memo":      cty.StringVal(body.Memo),
		"alert":     cty.ObjectVal(body.Alert.MarshalCTYValues()),
	}
	if body.Host != nil {
		values["host"] = cty.ObjectVal(body.Host.MarshalCTYValues())
	}
	if body.Service != nil {
		values["service"] = cty.ObjectVal(body.Service.MarshalCTYValues())
	}
	return values
}

type Host struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	URL       string  `json:"url"`
	Type      string  `json:"type,omitempty"`
	Status    string  `json:"status"`
	Memo      string  `json:"memo"`
	IsRetired bool    `json:"isRetired"`
	Roles     []*Role `json:"roles"`
}

func (body *Host) MarshalCTYValues() map[string]cty.Value {
	values := map[string]cty.Value{
		"id":         cty.StringVal(body.ID),
		"name":       cty.StringVal(body.Name),
		"url":        cty.StringVal(body.URL),
		"type":       cty.StringVal(body.Type),
		"status":     cty.StringVal(body.Status),
		"memo":       cty.StringVal(body.Memo),
		"is_retired": cty.BoolVal(body.IsRetired),
	}
	if len(body.Roles) == 0 {
		return values
	}
	roles := make([]cty.Value, 0, len(body.Roles))
	for _, role := range body.Roles {
		roles = append(roles, cty.ObjectVal(role.MarshalCTYValues()))
	}
	values["roles"] = cty.ListVal(roles)
	return values
}

type Role struct {
	Fullname    string `json:"fullname"`
	ServiceName string `json:"serviceName"`
	ServiceURL  string `json:"serviceUrl"`
	RoleName    string `json:"roleName"`
	RoleURL     string `json:"roleUrl"`
}

func (body *Role) MarshalCTYValues() map[string]cty.Value {
	values := map[string]cty.Value{
		"fullname":     cty.StringVal(body.Fullname),
		"service_name": cty.StringVal(body.ServiceName),
		"service_url":  cty.StringVal(body.ServiceURL),
		"role_name":    cty.StringVal(body.RoleName),
		"role_url":     cty.StringVal(body.RoleURL),
	}
	return values
}

type Service struct {
	ID    string  `json:"id"`
	Memo  string  `json:"memo"`
	Name  string  `json:"name"`
	OrgID string  `json:"orgId"`
	Roles []*Role `json:"roles"`
}

func (body *Service) MarshalCTYValues() map[string]cty.Value {
	values := map[string]cty.Value{
		"id":     cty.StringVal(body.ID),
		"name":   cty.StringVal(body.Name),
		"memo":   cty.StringVal(body.Memo),
		"org_id": cty.StringVal(body.OrgID),
	}
	if len(body.Roles) == 0 {
		return values
	}
	roles := make([]cty.Value, 0, len(body.Roles))
	for _, role := range body.Roles {
		roles = append(roles, cty.ObjectVal(role.MarshalCTYValues()))
	}
	values["roles"] = cty.ListVal(roles)
	return values
}

type Alert struct {
	OpenedAt          int64    `json:"openedAt"`
	ClosedAt          int64    `json:"closedAt"`
	CreatedAt         int64    `json:"createdAt"`
	CriticalThreshold *float64 `json:"criticalThreshold,omitempty"`
	Duration          int64    `json:"duration"`
	IsOpen            bool     `json:"isOpen"`
	MetricLabel       string   `json:"metricLabel"`
	MetricValue       float64  `json:"metricValue"`
	MonitorName       string   `json:"monitorName"`
	MonitorOperator   string   `json:"monitorOperator"`
	Status            string   `json:"status"`
	Trigger           string   `json:"trigger"`
	ID                string   `json:"id"`
	URL               string   `json:"url"`
	WarningThreshold  *float64 `json:"warningThreshold,omitempty"`
}

func (body *Alert) MarshalCTYValues() map[string]cty.Value {
	values := map[string]cty.Value{
		"opened_at":        cty.NumberIntVal(body.OpenedAt),
		"closed_at":        cty.NumberIntVal(body.ClosedAt),
		"created_at":       cty.NumberIntVal(body.CreatedAt),
		"duration":         cty.NumberIntVal(body.Duration),
		"is_open":          cty.BoolVal(body.IsOpen),
		"metric_label":     cty.StringVal(body.MetricLabel),
		"metric_value":     cty.NumberFloatVal(body.MetricValue),
		"monitor_name":     cty.StringVal(body.MonitorName),
		"monitor_operator": cty.StringVal(body.MonitorOperator),
		"status":           cty.StringVal(body.Status),
		"trigger":          cty.StringVal(body.Trigger),
		"id":               cty.StringVal(body.ID),
		"url":              cty.StringVal(body.URL),
	}
	if body.CriticalThreshold == nil {
		values["critical_threshold"] = cty.NullVal(cty.Number)
	} else {
		values["critical_threshold"] = cty.NumberFloatVal(*body.CriticalThreshold)
	}
	if body.WarningThreshold == nil {
		values["warning_threshold"] = cty.NullVal(cty.Number)
	} else {
		values["warning_threshold"] = cty.NumberFloatVal(*body.WarningThreshold)
	}
	return values
}
