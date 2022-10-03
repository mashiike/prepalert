package prepalert

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"
	"text/template"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/mashiike/prepalert/internal/funcs"
	"github.com/mashiike/prepalert/queryrunner"
	"golang.org/x/sync/errgroup"
)

type Rule struct {
	ruleName     string
	monitorName  string
	anyAlert     bool
	whenClosed   bool
	whenOpened   bool
	queries      []queryrunner.PreparedQuery
	infoTamplate *template.Template
	params       interface{}
}

func NewRule(client *mackerel.Client, cfg *hclconfig.RuleBlock) (*Rule, error) {
	var name string
	var anyAlert bool
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
	queries := make([]queryrunner.PreparedQuery, 0, len(cfg.Queries))
	for _, query := range cfg.Queries {
		queries = append(queries, query)
	}
	infoTemplate, err := template.New("info_template").Funcs(funcs.InfomationTemplateFuncMap).Parse(cfg.Infomation)
	if err != nil {
		return nil, fmt.Errorf("parse info template:%w", err)
	}
	rule := &Rule{
		ruleName:     cfg.Name,
		monitorName:  name,
		anyAlert:     anyAlert,
		whenClosed:   *cfg.Alert.WhenClosed,
		whenOpened:   *cfg.Alert.WhenOpened,
		queries:      queries,
		infoTamplate: infoTemplate,
		params:       cfg.Params,
	}
	return rule, nil
}

func (rule *Rule) Match(body *WebhookBody) bool {
	if rule.anyAlert {
		return true
	}
	return body.Alert.MonitorName == rule.monitorName
}

type QueryData struct {
	*WebhookBody
	Params interface{}
}

type RenderInfomationData struct {
	*WebhookBody
	QueryResults map[string]*queryrunner.QueryResult
	Params       interface{}
}

func (rule *Rule) BuildInfomation(ctx context.Context, body *WebhookBody) (string, error) {
	reqID := "-"
	info, ok := queryrunner.GetQueryRunningContext(ctx)
	if ok {
		reqID = fmt.Sprintf("%d", info.ReqID)
	}
	eg, egctx := errgroup.WithContext(ctx)
	queryData := &QueryData{
		WebhookBody: body,
		Params:      rule.params,
	}
	var queryResults sync.Map
	for _, query := range rule.queries {
		_query := query
		eg.Go(func() error {
			log.Printf("[info][%s] start run query name=%s", reqID, _query.Name())
			result, err := _query.Run(egctx, queryData)
			if err != nil {
				log.Printf("[error][%s]failed run query name=%s", reqID, _query.Name())
				return fmt.Errorf("query `%s`:%w", _query.Name(), err)
			}
			log.Printf("[info][%s] end run query name=%s", reqID, _query.Name())
			queryResults.Store(_query.Name(), result)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return "", err
	}
	data := &RenderInfomationData{
		WebhookBody:  body,
		QueryResults: make(map[string]*queryrunner.QueryResult, len(rule.queries)),
		Params:       rule.params,
	}
	queryResults.Range(func(key any, value any) bool {
		name, ok := key.(string)
		if !ok {
			log.Printf("[warn][%s] key=%v is not string", reqID, key)
			return false
		}
		queryResult, ok := value.(*queryrunner.QueryResult)
		if !ok {
			log.Printf("[warn][%s] value=%v is not *QueryResult", reqID, value)
			return false
		}
		data.QueryResults[name] = queryResult
		return true
	})
	return rule.RenderInfomation(ctx, data)
}

func (rule *Rule) RenderInfomation(ctx context.Context, data *RenderInfomationData) (string, error) {
	var buf bytes.Buffer
	if err := rule.infoTamplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (rule *Rule) Name() string {
	return rule.ruleName
}
