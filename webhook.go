package prepalert

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mackerelio/mackerel-client-go"
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

type Role struct {
	Fullname    string `json:"fullname"`
	ServiceName string `json:"serviceName"`
	ServiceURL  string `json:"serviceUrl"`
	RoleName    string `json:"roleName"`
	RoleURL     string `json:"roleUrl"`
}

type Service struct {
	ID    string  `json:"id"`
	Memo  string  `json:"memo"`
	Name  string  `json:"name"`
	OrgID string  `json:"orgId"`
	Roles []*Role `json:"roles"`
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

func (app *App) ProcessRule(ctx context.Context, rule *Rule, body *WebhookBody) error {
	reqID := "-"
	info, ok := GetHandleContext(ctx)
	if ok {
		reqID = fmt.Sprintf("%d", info.ReqID)
	}
	memo, err := rule.BuildMemo(ctx, body)
	if err != nil {
		return err
	}
	log.Printf("[debug][%s] memo: %s", reqID, memo)
	findOffset := int64(15 * time.Minute / time.Second)
	annotations, err := app.client.FindGraphAnnotations(app.service, body.Alert.OpenedAt-findOffset, body.Alert.ClosedAt+findOffset)
	if err != nil {
		return fmt.Errorf("find graph annotations: %w", err)
	}
	title := fmt.Sprintf("prepalert alert_id=%s", body.Alert.ID)
	description := fmt.Sprintf("related alert: %s\n\n%s", body.Alert.URL, memo)
	service := app.service
	if len(description) > 1024 {
		log.Printf("[warn][%s] description is too long length=%d, full description:%s", reqID, len(description), description)
		description = description[0:1020] + "\n..."
	}
	for _, annotation := range annotations {
		log.Printf("[debug][%s] check annotation id=%s title=%s", reqID, annotation.ID, annotation.Title)
		if annotation.Title == title {
			log.Printf("[info][%s] annotation is aleady exists, overwrite description: annotation_id=%s, alert_id=%s", reqID, annotation.ID, body.Alert.ID)
			annotation.Description = description
			annotation.Service = service
			_, err := app.client.UpdateGraphAnnotation(annotation.ID, &annotation)
			if err != nil {
				return fmt.Errorf("update graph annotations: %w", err)
			}
			return nil
		}
	}
	log.Printf("[info][%s] create new annotation: alert_id=%s", reqID, body.Alert.ID)
	annotation := &mackerel.GraphAnnotation{
		Title:       title,
		Description: description,
		From:        body.Alert.OpenedAt,
		To:          body.Alert.ClosedAt,
		Service:     service,
	}
	output, err := app.client.CreateGraphAnnotation(annotation)
	if err != nil {
		return fmt.Errorf("create graph annotations: %w", err)
	}
	log.Printf("[info][%s] annotation created annotation_id=%s", reqID, output.ID)
	return nil
}
