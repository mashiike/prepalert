package cloudwatchlogsinsights

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	cloudwatchlogsinsightsdriver "github.com/mashiike/cloudwatch-logs-insights-driver"
	"github.com/mashiike/prepalert/provider"
	"github.com/mashiike/prepalert/provider/sqlprovider"
)

var (
	defaultStartTimeExpr hcl.Expression
	defaultEndTimeExpr   hcl.Expression
)

func init() {
	provider.RegisterProvider("cloudwatch_logs_insights", NewProvider)
	var diags hcl.Diagnostics
	defaultStartTimeExpr, diags = hclsyntax.ParseExpression([]byte(`webhook.alert.opened_at - duration("15m")`), "start_time.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		panic(diags)
	}
	defaultEndTimeExpr, diags = hclsyntax.ParseExpression([]byte(`coalesce(webhook.alert.closed_at,now())`), "end_time.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		panic(diags)
	}
}

type Provider struct {
	Type string
	Name string
	ProviderParameter
	DSN string
	*sqlprovider.Provider
}

type ProviderParameter struct {
	Timeout              int64    `json:"timeout,omitempty"`
	Polling              int64    `json:"polling_interval,omitempty"`
	DefaultLogGroupNames []string `json:"default_log_group_names,omitempty"`
	Region               string   `json:"region,omitempty"`
	Limit                *int32   `json:"limit,omitempty"`
}

func NewProvider(pp *provider.ProviderParameter) (*Provider, error) {
	p := &Provider{
		Type: pp.Type,
		Name: pp.Name,
		ProviderParameter: ProviderParameter{
			Region: os.Getenv("AWS_REGION"),
		},
	}
	if err := json.Unmarshal(pp.Params, &p.ProviderParameter); err != nil {
		return nil, fmt.Errorf("failed to parse params: %w", err)
	}
	cfg := &cloudwatchlogsinsightsdriver.CloudwatchLogsInsightsConfig{
		Limit:         p.Limit,
		LogGroupNames: p.DefaultLogGroupNames,
		Region:        p.Region,
		Timeout:       time.Duration(p.Timeout) * time.Second,
		Polling:       time.Duration(p.Polling) * time.Second,
	}
	p.DSN = cfg.String()
	sqlp, err := sqlprovider.NewProvider("cloudwatch-logs-insights", p.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to create sql provider: %w", err)
	}
	sqlp.StatementAttributeName = "query"
	sqlp.ParametersAttributeName = ""
	p.Provider = sqlp
	return p, nil
}

type QueryParamter struct {
	StartTime     hcl.Expression `hcl:"start_time,optional"`
	EndTime       hcl.Expression `hcl:"end_time,optional"`
	Limit         *int32         `hcl:"limit,optional"`
	LogGroupNames hcl.Expression `hcl:"log_group_names,optional"`
	Remain        hcl.Body       `hcl:",remain"`
}

type Query struct {
	Provder   *Provider
	Parameter QueryParamter
	*sqlprovider.Query
}

func (p *Provider) NewQuery(name string, body hcl.Body, evalCtx *hcl.EvalContext) (provider.Query, error) {
	var params QueryParamter
	if diags := gohcl.DecodeBody(body, evalCtx, &params); diags.HasErrors() {
		return nil, diags
	}
	if params.StartTime == nil {
		params.StartTime = defaultStartTimeExpr
	}
	if params.EndTime == nil {
		params.EndTime = defaultEndTimeExpr
	}
	q, err := p.Provider.NewQuery(name, params.Remain, evalCtx)
	if err != nil {
		return nil, err
	}
	return &Query{
		Provder:   p,
		Parameter: params,
		Query:     q.(*sqlprovider.Query),
	}, nil
}

func (q *Query) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*provider.QueryResult, error) {
	var params []interface{}
	var startTime, endTime int64
	if dias := gohcl.DecodeExpression(q.Parameter.StartTime, evalCtx, &startTime); dias.HasErrors() {
		return nil, fmt.Errorf("failed to decode start_time: %w", dias)
	}
	if dias := gohcl.DecodeExpression(q.Parameter.EndTime, evalCtx, &endTime); dias.HasErrors() {
		return nil, fmt.Errorf("failed to decode end_time: %w", dias)
	}
	params = append(params,
		sql.Named("start_time", time.Unix(startTime, 0).UTC()),
		sql.Named("end_time", time.Unix(endTime, 0).UTC()),
	)
	var logGroupNames []string
	if q.Parameter.LogGroupNames != nil {
		if err := gohcl.DecodeExpression(q.Parameter.LogGroupNames, evalCtx, &logGroupNames); err != nil {
			return nil, fmt.Errorf("failed to decode log_group_names: %w", err)
		}
	} else {
		logGroupNames = q.Provder.DefaultLogGroupNames
	}
	for _, lg := range logGroupNames {
		params = append(params, sql.Named("log_group_name", lg))
	}
	if q.Parameter.Limit != nil {
		params = append(params, sql.Named("limit", *q.Parameter.Limit))
	}
	qr, err := q.Query.RunWithParamters(ctx, evalCtx, params)
	if err != nil {
		return nil, err
	}
	return qr, nil
}
