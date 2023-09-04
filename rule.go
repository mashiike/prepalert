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
	"golang.org/x/sync/errgroup"
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

func (rule *Rule) BuildInformation(ctx context.Context, evalCtx *hcl.EvalContext, body *WebhookBody) (string, error) {
	eg, egctx := errgroup.WithContext(ctx)
	builder := &EvalContextBuilder{
		Parent: evalCtx,
		Runtime: &RuntimeVariables{
			Params:       rule.params,
			Event:        body,
			QueryResults: make(map[string]*QueryResult),
		},
	}
	evalCtx, err := builder.Build()
	if err != nil {
		return "", fmt.Errorf("eval context builder: %w", err)
	}
	var queryResults sync.Map
	for _, query := range rule.queries {
		_query := query
		eg.Go(func() error {
			egctxWithQueryName := slogutils.With(
				egctx,
				"query_name", _query.Name(),
			)
			slog.InfoContext(egctxWithQueryName, "start run query")
			result, err := _query.Run(egctx, evalCtx.Variables, nil)
			if err != nil {
				slog.ErrorContext(egctxWithQueryName, "failed run query", "error", err.Error())
				return fmt.Errorf("query `%s`:%w", _query.Name(), err)
			}
			slog.InfoContext(egctxWithQueryName, "end run query")
			queryResults.Store(_query.Name(), result)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return "", err
	}
	queryResults.Range(func(key any, value any) bool {
		name, ok := key.(string)
		if !ok {
			slog.WarnContext(ctx,
				"failed fetch query results",
				"error", "key is not string",
				"key", fmt.Sprintf("%v", key),
				"key_type", fmt.Sprintf("%T", key),
			)
			return false
		}
		queryResult, ok := value.(*queryrunner.QueryResult)
		if !ok {
			slog.WarnContext(ctx,
				"failed fetch query results",
				"error", "value is not *QueryResult",
				"key", key,
				"value", fmt.Sprintf("%v", value),
				"value_type", fmt.Sprintf("%T", value),
			)
			return false
		}
		builder.Runtime.QueryResults[name] = (*QueryResult)(queryResult)
		return true
	})
	evalCtx, err = builder.Build()
	if err != nil {
		return "", err
	}
	return rule.RenderInformation(evalCtx)
}

func (rule *Rule) RenderInformation(evalCtx *hcl.EvalContext) (string, error) {
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
	info, err := rule.BuildInformation(ctx, evalCtx, body)
	if err != nil {
		return err
	}
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
