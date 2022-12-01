package prepalert

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/queryrunner"
	"github.com/zclconf/go-cty/cty"
)

type WebhookBody struct {
	OrgName  string   `json:"orgName"`
	Event    string   `json:"event"`
	ImageURL string   `json:"imageUrl"`
	Memo     string   `json:"memo"`
	Host     *Host    `json:"host,omitempty"`
	Service  *Service `json:"service,omitempty"`
	Alert    *Alert   `json:"alert"`
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
	Type      string  `json:"type"`
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
	OpenedAt          int64   `json:"openedAt"`
	ClosedAt          int64   `json:"closedAt"`
	CreatedAt         int64   `json:"createdAt"`
	CriticalThreshold float64 `json:"criticalThreshold"`
	Duration          int64   `json:"duration"`
	IsOpen            bool    `json:"isOpen"`
	MetricLabel       string  `json:"metricLabel"`
	MetricValue       float64 `json:"metricValue"`
	MonitorName       string  `json:"monitorName"`
	MonitorOperator   string  `json:"monitorOperator"`
	Status            string  `json:"status"`
	Trigger           string  `json:"trigger"`
	ID                string  `json:"id"`
	URL               string  `json:"url"`
	WarningThreshold  float64 `json:"warningThreshold"`
}

func (body *Alert) MarshalCTYValues() map[string]cty.Value {
	values := map[string]cty.Value{
		"opened_at":          cty.NumberIntVal(body.OpenedAt),
		"closed_at":          cty.NumberIntVal(body.ClosedAt),
		"created_at":         cty.NumberIntVal(body.CreatedAt),
		"critical_threshold": cty.NumberFloatVal(body.CriticalThreshold),
		"duration":           cty.NumberIntVal(body.Duration),
		"is_open":            cty.BoolVal(body.IsOpen),
		"metric_label":       cty.StringVal(body.MetricLabel),
		"metric_value":       cty.NumberFloatVal(body.MetricValue),
		"monitor_name":       cty.StringVal(body.MonitorName),
		"monitor_operator":   cty.StringVal(body.MonitorOperator),
		"status":             cty.StringVal(body.Status),
		"trigger":            cty.StringVal(body.Trigger),
		"id":                 cty.StringVal(body.ID),
		"url":                cty.StringVal(body.URL),
		"warning_threshold":  cty.NumberFloatVal(body.WarningThreshold),
	}
	return values
}

func (app *App) ProcessRule(ctx context.Context, rule *Rule, body *WebhookBody) error {
	reqID := queryrunner.GetRequestID(ctx)
	info, err := rule.BuildInfomation(ctx, app.evalCtx.NewChild(), body)
	if err != nil {
		return err
	}
	log.Printf("[debug][%s] infomation: %s", reqID, info)
	description := fmt.Sprintf("related alert: %s\n\n%s", body.Alert.URL, info)
	var showDetailsURL string
	var abbreviatedMessage string = "\n..."
	if app.EnableBackend() {
		evalCtx := app.evalCtx.NewChild()
		evalCtx.Variables = map[string]cty.Value{
			"runtime": cty.ObjectVal(map[string]cty.Value{
				"event": cty.ObjectVal(body.MarshalCTYValues()),
			}),
		}
		expr := *app.backend.ObjectKeyTemplate
		objectKeyTemplateValue, diags := expr.Value(evalCtx)
		if diags.HasErrors() {
			return fmt.Errorf("eval object key template: %w", diags)
		}
		if objectKeyTemplateValue.Type() != cty.String {
			return errors.New("object key template is not string")
		}
		if !objectKeyTemplateValue.IsKnown() {
			return errors.New("object key template is unknown")
		}
		objectKey := filepath.Join(*app.backend.ObjectKeyPrefix, objectKeyTemplateValue.AsString(), fmt.Sprintf("%s_%s.txt", body.Alert.ID, rule.Name()))
		u := app.backend.ViewerBaseURL.JoinPath(objectKeyTemplateValue.AsString(), fmt.Sprintf("%s_%s.txt", body.Alert.ID, rule.Name()))
		showDetailsURL = u.String()
		abbreviatedMessage = fmt.Sprintf("\nshow details: %s", showDetailsURL)
		log.Printf("[debug][%s] show details url `%s`", reqID, showDetailsURL)
		log.Printf("[debug][%s] try upload descriptsion to `s3://%s/%s`", reqID, app.backend.BucketName, objectKey)
		output, err := app.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: aws.String(app.backend.BucketName),
			Key:    aws.String(objectKey),
			Body:   strings.NewReader(description),
		})
		if err != nil {
			return fmt.Errorf("upload description failed: %w", err)
		}
		log.Printf("[info][%s] upload_location=%s", reqID, output.Location)
	}
	var wg sync.WaitGroup
	var errNum int32
	if rule.UpdateAlertMemo() {
		memo := info
		maxSize := rule.MaxAlertMemoSize()
		if len(memo) > maxSize {
			if app.EnableBackend() {
				log.Printf("[warn][%s] memo is too long length=%d, backend_url=%s", reqID, len(memo), showDetailsURL)
			} else {
				log.Printf("[warn][%s] memo is too long length=%d, full memo:%s", reqID, len(memo), memo)
			}
			if len(abbreviatedMessage) >= maxSize {
				memo = abbreviatedMessage[0:maxSize]
			} else {
				memo = memo[0:maxSize-len(abbreviatedMessage)] + abbreviatedMessage
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := app.UpdateAlertMemo(ctx, body, memo); err != nil {
				log.Printf("[error][%s] %v", reqID, err)
				atomic.AddInt32(&errNum, 1)
			}
		}()
	}
	if rule.PostGraphAnnotation() {
		maxSize := rule.MaxGraphAnnotationDescriptionSize()
		if len(description) > maxSize {
			if app.EnableBackend() {
				log.Printf("[warn][%s] description is too long length=%d, backend_url=%s", reqID, len(description), showDetailsURL)
			} else {
				log.Printf("[warn][%s] description is too long length=%d, full description:%s", reqID, len(description), description)
			}
			if len(abbreviatedMessage) >= maxSize {
				description = abbreviatedMessage[0:maxSize]
			} else {
				description = description[0:maxSize-len(abbreviatedMessage)] + abbreviatedMessage
			}
		}
		annotation := &mackerel.GraphAnnotation{
			Title:       fmt.Sprintf("prepalert alert_id=%s rule=%s", body.Alert.ID, rule.Name()),
			Description: description,
			From:        body.Alert.OpenedAt,
			To:          body.Alert.ClosedAt,
			Service:     app.service,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := app.PostGraphAnnotation(ctx, body, annotation); err != nil {
				log.Printf("[error][%s] %v", reqID, err)
				atomic.AddInt32(&errNum, 1)
			}
		}()
	}
	wg.Wait()
	if errNum != 0 {
		return fmt.Errorf("has %d errors", errNum)
	}
	return nil
}

func (app *App) PostGraphAnnotation(ctx context.Context, body *WebhookBody, params *mackerel.GraphAnnotation) error {
	reqID := queryrunner.GetRequestID(ctx)
	findOffset := int64(15 * time.Minute / time.Second)
	annotations, err := app.client.FindGraphAnnotations(app.service, body.Alert.OpenedAt-findOffset, body.Alert.ClosedAt+findOffset)
	if err != nil {
		return fmt.Errorf("find graph annotations: %w", err)
	}
	for _, annotation := range annotations {
		log.Printf("[debug][%s] check annotation id=%s title=%s", reqID, annotation.ID, annotation.Title)
		if annotation.Title == params.Title {
			log.Printf("[info][%s] annotation is aleady exists, overwrite description: annotation_id=%s, alert_id=%s", reqID, annotation.ID, body.Alert.ID)
			annotation.Description = params.Description
			annotation.Service = params.Service
			_, err := app.client.UpdateGraphAnnotation(annotation.ID, &annotation)
			if err != nil {
				return fmt.Errorf("update graph annotations: %w", err)
			}
			return nil
		}
	}
	log.Printf("[info][%s] create new annotation: alert_id=%s", reqID, body.Alert.ID)
	output, err := app.client.CreateGraphAnnotation(params)
	if err != nil {
		return fmt.Errorf("create graph annotations: %w", err)
	}
	log.Printf("[info][%s] annotation created annotation_id=%s", reqID, output.ID)
	return nil
}

func (app *App) UpdateAlertMemo(ctx context.Context, body *WebhookBody, memo string) error {
	reqID := queryrunner.GetRequestID(ctx)
	log.Printf("[info][%s] update alert memo: alert_id=%s", reqID, body.Alert.ID)
	_, err := app.client.UpdateAlert(body.Alert.ID, mackerel.UpdateAlertParam{
		Memo: memo,
	})
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	return nil
}

func (app *App) NewWebhookBody(ctx context.Context, alertID string) (*WebhookBody, error) {
	org, err := app.client.GetOrg()
	if err != nil {
		return nil, fmt.Errorf("get org:%w", err)
	}
	alert, err := app.client.GetAlert(alertID)
	if err != nil {
		return nil, fmt.Errorf("get alert:%w", err)
	}
	monitor, err := app.client.GetMonitor(alert.MonitorID)
	if err != nil {
		return nil, fmt.Errorf("get monitor:%w", err)
	}
	body := &WebhookBody{
		OrgName: org.Name,
		Event:   "alert",
		Alert: &Alert{
			OpenedAt:          alert.OpenedAt,
			ClosedAt:          alert.ClosedAt,
			CreatedAt:         alert.OpenedAt * 1000,
			CriticalThreshold: 0,
			Duration:          0,
			IsOpen:            alert.Status != "OK",
			MetricLabel:       "",
			MetricValue:       0,
			MonitorName:       monitor.MonitorName(),
			MonitorOperator:   "",
			Status:            alert.Status,
			Trigger:           "monitor",
			ID:                alert.ID,
			URL:               fmt.Sprintf("https://mackerel.io/orgs/%s/alerts/%s", org.Name, alert.ID),
			WarningThreshold:  0,
		},
	}
	switch m := monitor.(type) {
	case *mackerel.MonitorConnectivity:
		body.Memo = m.Memo
	case *mackerel.MonitorHostMetric:
		body.Memo = m.Memo
		if m.Warning != nil {
			body.Alert.WarningThreshold = *m.Warning
		}
		if m.Critical != nil {
			body.Alert.CriticalThreshold = *m.Critical
		}
		body.Alert.MonitorOperator = m.Operator
		body.Alert.Duration = int64(m.Duration)
		body.Alert.MetricLabel = m.Metric
		body.Host = &Host{}
	case *mackerel.MonitorServiceMetric:
		body.Memo = m.Memo
		if m.Warning != nil {
			body.Alert.WarningThreshold = *m.Warning
		}
		if m.Critical != nil {
			body.Alert.CriticalThreshold = *m.Critical
		}
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
		if m.Warning != nil {
			body.Alert.WarningThreshold = *m.Warning
		}
		if m.Critical != nil {
			body.Alert.CriticalThreshold = *m.Critical
		}
		body.Alert.MonitorOperator = m.Operator
	case *mackerel.MonitorAnomalyDetection:
		body.Memo = m.Memo
	default:
		return nil, fmt.Errorf("unknown monitor type: %s", m.MonitorName())
	}

	return body, nil
}
