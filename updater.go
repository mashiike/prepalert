package prepalert

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/Songmu/flextime"
	"github.com/hashicorp/hcl/v2"
	"github.com/mackerelio/mackerel-client-go"
)

type MackerelUpdater struct {
	svc                    *MackerelService
	mu                     sync.Mutex
	backend                Backend
	body                   *WebhookBody
	memoSectionText        []string
	memoSectionSizeLimit   []*int
	additionalDescriptions map[string][]string
	postServices           map[string]struct{}
}

func (svc *MackerelService) NewMackerelUpdater(body *WebhookBody, backend Backend) *MackerelUpdater {
	return &MackerelUpdater{
		svc:                    svc,
		body:                   body,
		backend:                backend,
		memoSectionText:        make([]string, 0),
		memoSectionSizeLimit:   make([]*int, 0),
		additionalDescriptions: make(map[string][]string),
		postServices:           make(map[string]struct{}),
	}
}

func (u *MackerelUpdater) AddMemoSectionText(text string, sizeLimit *int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.memoSectionText = append(u.memoSectionText, text)
	u.memoSectionSizeLimit = append(u.memoSectionSizeLimit, sizeLimit)
}

func (u *MackerelUpdater) AddService(service string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.postServices[service] = struct{}{}
	if _, ok := u.additionalDescriptions[service]; !ok {
		u.additionalDescriptions[service] = make([]string, 0)
	}
}

func (u *MackerelUpdater) AddAdditionalDescription(service string, text string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.additionalDescriptions[service] = append(u.additionalDescriptions[service], text)
}

func (u *MackerelUpdater) Flush(ctx context.Context, evalCtx *hcl.EvalContext) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	body := u.body
	if len(u.memoSectionText) > 0 {
		var fullText, memo string
		for i, text := range u.memoSectionText {
			fullText += "\n\n" + text
			if u.memoSectionSizeLimit[i] != nil {
				text = triming(text, *u.memoSectionSizeLimit[i], "\n...")
			}
			memo += "\n\n" + text
		}
		fullText = strings.TrimPrefix(fullText, "\n\n")
		memo = strings.TrimPrefix(memo, "\n\n")
		uploadBody := strings.NewReader(fmt.Sprintf("related alert: %s\n\n%s", body.Alert.URL, fullText))
		fullTextURL, uploaded, err := u.backend.Upload(ctx, evalCtx, body.Alert.ID, uploadBody)
		if err != nil {
			return fmt.Errorf("upload to backend:%w", err)
		}
		if uploaded {
			slog.DebugContext(ctx, "uploaded to backend", "full_text_url", fullTextURL)
			memo = fmt.Sprintf("Full Text URL: %s\n\n%s", fullTextURL, memo)
		} else {
			memo = fullText
		}
		header := "## Prepalert\n"
		memo = header + memo
		alert, err := u.svc.GetAlertWithCache(ctx, body.Alert.ID)
		if err != nil {
			return fmt.Errorf("get alert: %w", err)
		}
		if alert.Memo != "" {
			currentSection := extructSection(alert.Memo, header)
			if currentSection != "" {
				memo = strings.ReplaceAll(alert.Memo, currentSection, memo)
			} else {
				memo = alert.Memo + "\n\n" + memo
			}
		}
		if len(memo) > AlertMemoMaxSize {
			slog.WarnContext(
				ctx,
				"alert memo is too long",
				"length", len(memo),
				"full_text_url", fullTextURL,
			)
			slog.DebugContext(
				ctx,
				"alert memo is too long",
				"memo", memo,
			)
			memo = triming(memo, AlertMemoMaxSize, "\n...")
		}
		err = u.svc.UpdateAlertMemo(ctx, body.Alert.ID, memo)
		if err != nil {
			return fmt.Errorf("update alert memo: %w", err)
		}
	}
	errs := make([]error, 0, 2)
	if len(u.postServices) > 0 {
		to := flextime.Now().Unix()
		if body.Alert.ClosedAt != nil {
			to = *body.Alert.ClosedAt
		}
		for service := range u.postServices {
			description := fmt.Sprintf("related alert: %s\n", body.Alert.URL)
			for _, text := range u.additionalDescriptions[service] {
				description += text + "\n"
			}
			slog.DebugContext(ctx, "dump description", "description", description)
			err := u.svc.PostGraphAnnotation(ctx, &mackerel.GraphAnnotation{
				Title:       fmt.Sprintf("prepalert alert_id=%s", body.Alert.ID),
				Description: description,
				From:        body.Alert.OpenedAt,
				To:          to,
				Service:     service,
			})
			if err != nil {
				errs = append(errs, fmt.Errorf("post graph annotation: %w", err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("post graph annotation failed: %v", errs)
	}
	return nil
}
