package prepalert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/mashiike/queryrunner"
	"github.com/mashiike/slogutils"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/sync/errgroup"
)

type Rule struct {
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
}

func NewRule(client *mackerel.Client, cfg *hclconfig.RuleBlock) (*Rule, error) {
	var name string
	var anyAlert, onClosed, onOpened bool
	if cfg.Alert.MonitorID != nil {
		m, err := client.GetMonitor(*cfg.Alert.MonitorID)
		if err != nil {
			return nil, fmt.Errorf("get monitor from mackerel:%w", err)
		}
		name = m.MonitorName()
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
