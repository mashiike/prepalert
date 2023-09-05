package prepalert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/hcl/v2"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/mashiike/queryrunner"
	"github.com/mashiike/slogutils"
	"github.com/zclconf/go-cty/cty"
)

type Rule struct {
	svc                               *MackerelService
	backend                           Backend
	ruleName                          string
	monitorName                       string
	anyAlert                          bool
	onClosed                          bool
	onOpened                          bool
	queries                           []queryrunner.PreparedQuery
	information                       hcl.Expression
	params                            cty.Value
	postGraphAnnotation               bool
	updateAlertMemo                   bool
	maxGraphAnnotationDescriptionSize *int
	maxAlertMemoSize                  *int
	service                           string
}

func NewRule(svc *MackerelService, backend Backend, cfg *hclconfig.RuleBlock, service string) (*Rule, error) {
	var name string
	var anyAlert, onClosed, onOpened bool
	if cfg.Alert.MonitorID != nil {
		var err error
		name, err = svc.GetMonitorName(context.Background(), *cfg.Alert.MonitorID)
		if err != nil {
			return nil, fmt.Errorf("get monitor name:%w", err)
		}
	}
	if cfg.Alert.MonitorName != nil {
		name = *cfg.Alert.MonitorName
	}
	if cfg.Alert.Any != nil {
		anyAlert = *cfg.Alert.Any
	}
	if cfg.Alert.OnOpened != nil {
		onOpened = *cfg.Alert.OnOpened
	}
	if cfg.Alert.OnClosed != nil {
		onClosed = *cfg.Alert.OnClosed
	} else {
		onClosed = true
	}
	queries := make([]queryrunner.PreparedQuery, 0, len(cfg.Queries))
	for _, query := range cfg.Queries {
		queries = append(queries, query)
	}
	rule := &Rule{
		svc:                 svc,
		backend:             backend,
		ruleName:            cfg.Name,
		monitorName:         name,
		anyAlert:            anyAlert,
		onOpened:            onOpened,
		onClosed:            onClosed,
		queries:             queries,
		information:         cfg.Information,
		params:              cfg.Params,
		postGraphAnnotation: cfg.PostGraphAnnotation,
		updateAlertMemo:     cfg.UpdateAlertMemo,
		service:             service,
	}
	return rule, nil
}

func (rule *Rule) Match(body *WebhookBody) bool {
	if rule.anyAlert {
		return true
	}
	if body.Alert.IsOpen {
		if !rule.onOpened {
			return false
		}
	} else {
		if !rule.onClosed {
			return false
		}
	}
	return body.Alert.MonitorName == rule.monitorName
}
func (rule *Rule) Name() string {
	return rule.ruleName
}

func (rule *Rule) PostGraphAnnotation() bool {
	return rule.postGraphAnnotation
}

func (rule *Rule) UpdateAlertMemo() bool {
	return rule.updateAlertMemo
}

const (
	maxDescriptionSize = 1024
	maxMemoSize        = 80 * 1000
	defualtMaxMemoSize = 1024
)

func (rule *Rule) MaxGraphAnnotationDescriptionSize() int {
	if rule.maxGraphAnnotationDescriptionSize == nil {
		return maxDescriptionSize
	}
	if *rule.maxGraphAnnotationDescriptionSize > maxDescriptionSize {
		return maxDescriptionSize
	}
	if *rule.maxGraphAnnotationDescriptionSize <= 0 {
		return 100
	}
	return *rule.maxGraphAnnotationDescriptionSize
}

func (rule *Rule) MaxAlertMemoSize() int {
	if rule.maxAlertMemoSize == nil {
		return defualtMaxMemoSize
	}
	if *rule.maxAlertMemoSize > maxMemoSize {
		return maxMemoSize
	}
	if *rule.maxAlertMemoSize <= 0 {
		return 100
	}
	return *rule.maxAlertMemoSize
}

func (rule *Rule) Render(ctx context.Context, evalCtx *hcl.EvalContext, body *WebhookBody) error {
	ctx = slogutils.With(ctx, "rule_name", rule.Name())
	var wg, queryWg sync.WaitGroup
	resultCh := make(chan *queryrunner.QueryResult, len(rule.queries))
	builder := EvalContextBuilder{
		Parent: evalCtx,
		Runtime: &RuntimeVariables{
			Event:        body,
			Params:       rule.params,
			QueryResults: make(map[string]*QueryResult, len(rule.queries)),
		},
	}
	queryEvalCtx, err := builder.Build()
	if err != nil {
		return fmt.Errorf("eval context builder: %w", err)
	}
	var queryErrorCount int32
	for _, query := range rule.queries {
		builder.Runtime.QueryResults[query.Name()] = (*QueryResult)(queryrunner.NewQueryResult(
			query.Name(),
			"",
			[]string{"status"},
			[][]string{{"running"}},
		))
		wg.Add(1)
		queryWg.Add(1)
		go func(query queryrunner.PreparedQuery) {
			defer func() {
				wg.Done()
				queryWg.Done()
			}()
			egctxWithQueryName := slogutils.With(
				ctx,
				"query_name", query.Name(),
			)
			slog.InfoContext(egctxWithQueryName, "start run query")
			result, err := query.Run(egctxWithQueryName, queryEvalCtx.Variables, nil)
			if err != nil {
				slog.ErrorContext(egctxWithQueryName, "failed run query", "error", err.Error())
				atomic.AddInt32(&queryErrorCount, 1)
				resultCh <- queryrunner.NewQueryResult(
					query.Name(),
					"",
					[]string{"status", "error"},
					[][]string{{"failed", err.Error()}},
				)
				return
			}
			slog.InfoContext(egctxWithQueryName, "end run query")
			resultCh <- result
		}(query)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		queryWg.Wait()
		close(resultCh)
	}()
	f := func() error {
		evalCtx, err := builder.Build()
		if err != nil {
			return fmt.Errorf("eval context builder: %w", err)
		}
		info, err := rule.BuildInfomation(evalCtx)
		if err != nil {
			return fmt.Errorf("build information:%w", err)
		}
		if err := rule.render(ctx, evalCtx, body, info); err != nil {
			return fmt.Errorf("render:%w", err)
		}
		return nil
	}
	wg.Add(1)
	var bgRenderErr error
	var isRender bool
	go func() {
		defer wg.Done()
		for result := range resultCh {
			builder.Runtime.QueryResults[result.Name] = (*QueryResult)(result)
			if err := f(); err != nil {
				bgRenderErr = fmt.Errorf("failed render:%w", err)
			}
			isRender = true
		}
	}()
	wg.Wait()
	if bgRenderErr != nil {
		return bgRenderErr
	}
	if queryErrorCount > 0 {
		return fmt.Errorf("%s rule render failed", rule.Name())
	}
	if !isRender {
		return f()
	}
	return nil
}

func (rule *Rule) render(ctx context.Context, evalCtx *hcl.EvalContext, body *WebhookBody, info string) error {
	slog.DebugContext(ctx, "dump infomation", "infomation", info)
	description := fmt.Sprintf("related alert: %s\n\n%s", body.Alert.URL, info)
	showDetailsURL, uploaded, err := rule.backend.Upload(
		ctx, evalCtx,
		fmt.Sprintf("%s_%s", body.Alert.ID, rule.Name()),
		strings.NewReader(description),
	)
	if err != nil {
		return fmt.Errorf("upload to backend:%w", err)
	}
	var abbreviatedMessage string = "\n..."
	var wg sync.WaitGroup
	var errNum int32
	if rule.UpdateAlertMemo() {
		memo := info
		maxSize := rule.MaxAlertMemoSize()
		if len(memo) > maxSize {
			if uploaded {
				slog.WarnContext(
					ctx,
					"alert memo is too long",
					"length", len(memo),
					"show_details_url", showDetailsURL,
				)
			} else {
				slog.WarnContext(
					ctx,
					"alert memo is too long",
					"length", len(memo),
					"full_memo", memo,
				)
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
			if err := rule.svc.UpdateAlertMemo(ctx, body.Alert.ID, memo); err != nil {
				slog.ErrorContext(ctx, "failed update alert memo", "error", err.Error())
				atomic.AddInt32(&errNum, 1)
			}
		}()
	}

	if rule.PostGraphAnnotation() {
		maxSize := rule.MaxGraphAnnotationDescriptionSize()
		if len(description) > maxSize {
			if uploaded {
				slog.WarnContext(
					ctx,
					"graph anotation description is too long",
					"length", len(description),
					"show_details_url", showDetailsURL,
				)
			} else {
				slog.WarnContext(
					ctx,
					"graph anotation description is too long",
					"length", len(description),
					"full_description", description,
				)
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
			Service:     rule.service,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rule.svc.PostGraphAnnotation(ctx, annotation); err != nil {
				slog.ErrorContext(
					ctx,
					"failed post graph annotation",
					"error", err.Error(),
				)
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

func (rule *Rule) BuildInfomation(evalCtx *hcl.EvalContext) (string, error) {
	value, diags := rule.information.Value(evalCtx)
	if diags.HasErrors() {
		return "", diags
	}
	if value.Type() != cty.String {
		return "", errors.New("information is not string")
	}
	if value.IsNull() {
		return "", errors.New("information is nil")
	}
	if !value.IsKnown() {
		return "", errors.New("information is unknown")
	}
	return value.AsString(), nil
}
