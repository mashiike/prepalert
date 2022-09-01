package prepalert

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"
	"text/template"

	"github.com/mackerelio/mackerel-client-go"
	"golang.org/x/sync/errgroup"
)

type Rule struct {
	monitorName  string
	queries      []CompiledQuery
	memoTamplate *template.Template
}

func NewRule(client *mackerel.Client, cfg *RuleConfig, runners QueryRunners) (*Rule, error) {
	name := cfg.Monitor.Name
	if name == "" {
		m, err := client.GetMonitor(cfg.Monitor.ID)
		if err != nil {
			return nil, fmt.Errorf("get monitor from mackerel:%w", err)
		}
		name = m.MonitorName()
	}
	queries := make([]CompiledQuery, 0, len(cfg.Queries))
	for _, query := range cfg.Queries {
		runner, ok := runners.Get(query.Runner)
		if !ok {
			return nil, fmt.Errorf("queires[%s] runner `%s` not found", query.Name, query.Runner)
		}
		compiled, err := runner.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("queries[%s] compile faield:%w", query.Name, err)
		}
		queries = append(queries, compiled)
	}
	memoTemplate, err := template.New("memo_template").Funcs(memoTemplateFuncMap).Parse(cfg.Memo.Text)
	if err != nil {
		return nil, fmt.Errorf("parse memo template:%w", err)
	}
	rule := &Rule{
		monitorName:  name,
		queries:      queries,
		memoTamplate: memoTemplate,
	}
	return rule, nil
}

func (rule *Rule) Match(body *WebhookBody) bool {
	return body.Alert.MonitorName == rule.monitorName
}

func (rule *Rule) BuildMemo(ctx context.Context, body *WebhookBody) (string, error) {
	reqID := "-"
	info, ok := GetHandleContext(ctx)
	if ok {
		reqID = fmt.Sprintf("%d", info.ReqID)
	}
	eg, egctx := errgroup.WithContext(ctx)
	var queryResults sync.Map
	for _, query := range rule.queries {
		_query := query
		eg.Go(func() error {
			log.Printf("[info][%s] start run query name=%s", reqID, _query.Name())
			result, err := _query.Run(egctx, body)
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
	data := &RenderMemoData{
		WebhookBody:  body,
		QueryResults: make(map[string]*QueryResult, len(rule.queries)),
	}
	queryResults.Range(func(key any, value any) bool {
		name, ok := key.(string)
		if !ok {
			log.Printf("[warn][%s] key=%v is not string", reqID, key)
			return false
		}
		queryResult, ok := value.(*QueryResult)
		if !ok {
			log.Printf("[warn][%s] value=%v is not *QueryResult", reqID, value)
			return false
		}
		data.QueryResults[name] = queryResult
		return true
	})
	return rule.RenderMemo(ctx, data)
}

type RenderMemoData struct {
	*WebhookBody
	QueryResults map[string]*QueryResult
}

func (rule *Rule) RenderMemo(ctx context.Context, data *RenderMemoData) (string, error) {
	var buf bytes.Buffer
	if err := rule.memoTamplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
