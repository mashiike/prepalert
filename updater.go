package prepalert

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
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
	memoSectionNames       []string
	memoSectionText        map[string]string
	memoSectionSizeLimit   map[string]*int
	additionalDescriptions map[string][]string
	postServices           map[string]struct{}
}

func (svc *MackerelService) NewMackerelUpdater(body *WebhookBody, backend Backend) *MackerelUpdater {
	return &MackerelUpdater{
		svc:                    svc,
		body:                   body,
		backend:                backend,
		memoSectionNames:       make([]string, 0),
		memoSectionText:        make(map[string]string),
		memoSectionSizeLimit:   make(map[string]*int),
		additionalDescriptions: make(map[string][]string),
		postServices:           make(map[string]struct{}),
	}
}

func (u *MackerelUpdater) AddMemoSectionText(sectionName string, text string, sizeLimit *int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.memoSectionNames = append(u.memoSectionNames, sectionName)
	u.memoSectionText[sectionName] = text
	u.memoSectionSizeLimit[sectionName] = sizeLimit
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

const (
	prepalertSectionHeader = "## Prepalert"
	fullTextLabel          = "Full Text URL:"
)

var (
	// match `## Prepalert\n`` or `## Prepalert\nFull Text URL: <url>\n`
	prepalertHeaderRegexp = regexp.MustCompile(fmt.Sprintf(
		`(?m)^%s\n(?:%s .*\n)?`, prepalertSectionHeader, fullTextLabel,
	))
)

func (u *MackerelUpdater) Flush(ctx context.Context, evalCtx *hcl.EvalContext) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	body := u.body
	if len(u.memoSectionText) > 0 {
		alert, err := u.svc.GetAlertWithCache(ctx, body.Alert.ID)
		if err != nil {
			return fmt.Errorf("get alert: %w", err)
		}
		currentMemo := alert.Memo
		currentPrepalertSection := extructSection(currentMemo, prepalertSectionHeader)
		fullText := strings.Trim(prepalertHeaderRegexp.ReplaceAllString(currentPrepalertSection, ""), "\n")
		memo := fullText
		for _, sectionName := range u.memoSectionNames {
			extracted := extructSection(fullText, "### "+sectionName)
			sectionText := u.memoSectionText[sectionName]
			trimedSectionText := sectionText
			if u.memoSectionSizeLimit[sectionName] != nil {
				trimedSectionText = triming(sectionText, *u.memoSectionSizeLimit[sectionName], "\n...")
			}
			if extracted != "" {
				fullText = strings.ReplaceAll(fullText, extracted, "### "+sectionName+"\n\n"+sectionText)
				memo = strings.ReplaceAll(memo, extracted, "### "+sectionName+"\n\n"+trimedSectionText)

			} else {
				fullText += "\n\n### " + sectionName + "\n\n" + u.memoSectionText[sectionName]
				memo += "\n\n### " + sectionName + "\n\n" + trimedSectionText
			}
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
		memo = prepalertSectionHeader + "\n" + memo
		if currentPrepalertSection != "" {
			memo = strings.ReplaceAll(currentMemo, currentPrepalertSection, memo)
		} else {
			memo = alert.Memo + "\n\n" + memo
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
		memo = strings.Trim(memo, "\n") + "\n"
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
