package cloudwatchlogsinsights

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/prepalert/queryrunner"
	"github.com/samber/lo"
	"github.com/zclconf/go-cty/cty"
)

const TypeName = "cloudwatch_logs_insights"

func init() {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName:             TypeName,
		BuildQueryRunnerFunc: BuildQueryRunner,
	})
	if err != nil {
		panic(fmt.Errorf("register cloudwatch_logs_insights query runner:%w", err))
	}
	log.Println("[info] load cloudwatch_logs_insights query runner")
}

func BuildQueryRunner(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
	queryRunner := &QueryRunner{
		name: name,
	}
	diags := gohcl.DecodeBody(body, ctx, queryRunner)
	if diags.HasErrors() {
		return nil, diags
	}
	optFns := make([]func(*config.LoadOptions) error, 0)
	if queryRunner.Region != nil {
		optFns = append(optFns, config.WithRegion(*queryRunner.Region))
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {

		return nil, diags
	}
	queryRunner.client = cloudwatchlogs.NewFromConfig(awsCfg)
	return queryRunner, diags
}

func (r *QueryRunner) Name() string {
	return r.name
}

func (r *QueryRunner) Type() string {
	return TypeName
}

type QueryRunner struct {
	client *cloudwatchlogs.Client
	name   string
	Region *string `hcl:"region"`
}

type PreparedQuery struct {
	name   string
	runner *QueryRunner

	StartTime hcl.Expression `hcl:"start_time"`
	EndTime   hcl.Expression `hcl:"end_time"`
	Query     hcl.Expression `hcl:"query"`
	Limit     *int32         `hcl:"limit"`

	LogGroupName  *string  `hcl:"log_group_name"`
	LogGroupNames []string `hcl:"log_group_names,optional"`
	IgnoreFields  []string `hcl:"ignore_fields,optional"`

	Attrs hcl.Attributes `hcl:",body"`
}

func (r *QueryRunner) Prepare(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.PreparedQuery, hcl.Diagnostics) {
	log.Printf("[debug] prepare `%s` with cloudwatch_logs_insights query_runner", name)
	q := &PreparedQuery{
		name:   name,
		runner: r,
	}
	diags := gohcl.DecodeBody(body, ctx, q)
	if diags.HasErrors() {
		return nil, diags
	}

	log.Printf("[debug] end cloudwatch_logs_insights query block %d error diags", len(diags.Errs()))
	var logGroupNameRange, logGroupNamesRange *hcl.Range
	for _, attr := range q.Attrs {
		switch attr.Name {
		case "log_group_name":
			logGroupNameRange = attr.Range.Ptr()
		case "log_group_names":
			logGroupNamesRange = attr.Range.Ptr()
		}
	}
	if logGroupNameRange != nil && logGroupNamesRange != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid log_group_names",
			Detail:   fmt.Sprintf("log_group_name alerady declared on %s. required attribute log_group_name or log_group_names, but not both.", logGroupNameRange.String()),
			Subject:  logGroupNamesRange,
		})
	}
	if logGroupNameRange == nil && logGroupNamesRange == nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid log_group_name",
			Detail:   "required attribute log_group_name or log_group_names",
			Subject:  body.MissingItemRange().Ptr(),
		})
	}
	startTimeValue, _ := q.StartTime.Value(ctx)
	if startTimeValue.IsKnown() && startTimeValue.IsNull() {
		var parseDiags hcl.Diagnostics
		q.StartTime, parseDiags = hclsyntax.ParseExpression([]byte(`strftime_in_zone("%Y-%m-%dT%H:%M:%S%z", "UTC", runtime.event.alert.opened_at)`), "default_start_time.hcl", hcl.InitialPos)
		diags = append(diags, parseDiags...)
	}
	endTimeValue, _ := q.EndTime.Value(ctx)
	if endTimeValue.IsKnown() && endTimeValue.IsNull() {
		var parseDiags hcl.Diagnostics
		q.EndTime, parseDiags = hclsyntax.ParseExpression([]byte(`strftime_in_zone("%Y-%m-%dT%H:%M:%S%z", "UTC", runtime.event.alert.closed_at)`), "default_end_time.hcl", hcl.InitialPos)
		diags = append(diags, parseDiags...)
	}
	return q, diags
}

func (q *PreparedQuery) Name() string {
	return q.name
}

const layout = "2006-01-02T15:04:05-0700"

func (q *PreparedQuery) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*queryrunner.QueryResult, error) {
	queryValue, diags := q.Query.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !queryValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "query is unknown",
			Subject:  q.Query.Range().Ptr(),
		})
		return nil, diags
	}
	if queryValue.Type() != cty.String {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "query is not string",
			Subject:  q.Query.Range().Ptr(),
		})
		return nil, diags
	}
	query := queryValue.AsString()
	if query == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "query is empty",
			Subject:  q.Query.Range().Ptr(),
		})
		return nil, diags
	}

	startTimeValue, diags := q.StartTime.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !startTimeValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid start_time template",
			Detail:   "start_time is unknown",
			Subject:  q.StartTime.Range().Ptr(),
		})
		return nil, diags
	}
	if startTimeValue.Type() != cty.String {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid start_time template",
			Detail:   "start_time is not string",
			Subject:  q.StartTime.Range().Ptr(),
		})
		return nil, diags
	}
	startTime, err := time.Parse(layout, startTimeValue.AsString())
	if err != nil {
		return nil, fmt.Errorf("parse start_time: %w", err)
	}

	endTimeValue, diags := q.EndTime.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !endTimeValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid end_time template",
			Detail:   "end_time is unknown",
			Subject:  q.EndTime.Range().Ptr(),
		})
		return nil, diags
	}
	if endTimeValue.Type() != cty.String {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid end_time template",
			Detail:   "end_time is not string",
			Subject:  q.EndTime.Range().Ptr(),
		})
		return nil, diags
	}
	endTime, err := time.Parse(layout, endTimeValue.AsString())
	if err != nil {
		return nil, fmt.Errorf("parse end_time: %w", err)
	}

	params := &cloudwatchlogs.StartQueryInput{
		StartTime:     aws.Int64(startTime.Unix()),
		EndTime:       aws.Int64(endTime.Unix()),
		QueryString:   aws.String(query),
		Limit:         q.Limit,
		LogGroupName:  q.LogGroupName,
		LogGroupNames: q.LogGroupNames,
	}
	return q.runner.RunQuery(ctx, q.name, params, q.IgnoreFields)
}

func (r *QueryRunner) RunQuery(ctx context.Context, name string, params *cloudwatchlogs.StartQueryInput, ignoreFields []string) (*queryrunner.QueryResult, error) {
	reqID := "-"
	hctx, ok := queryrunner.GetQueryRunningContext(ctx)
	if ok {
		reqID = fmt.Sprintf("%d", hctx.ReqID)
	}
	startQueryOutput, err := r.client.StartQuery(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("start_query: %w", err)
	}
	var logGroupNames string
	if params.LogGroupName != nil {
		logGroupNames = *params.LogGroupName
	}
	if params.LogGroupNames != nil {
		logGroupNames = "[" + strings.Join(params.LogGroupNames, ",") + "]"
	}
	log.Printf("[info][%s] start cloudwatch logs insights query to %s", reqID, logGroupNames)
	log.Printf("[info][%s] time range: %s ~ %s", reqID, time.Unix(*params.StartTime, 0).In(time.Local), time.Unix(*params.EndTime, 0).In(time.Local))
	log.Printf("[debug][%s] query string: %s", reqID, *params.QueryString)
	queryStart := time.Now()
	getQueryResultOutput, err := r.waitQueryResult(ctx, queryStart, &cloudwatchlogs.GetQueryResultsInput{
		QueryId: startQueryOutput.QueryId,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("[debug][%s] query result: %d results, %s scanned, %f records matched, %f recoreds scanned",
		reqID,
		len(getQueryResultOutput.Results),
		humanize.Bytes(uint64(getQueryResultOutput.Statistics.BytesScanned)),
		getQueryResultOutput.Statistics.RecordsMatched,
		getQueryResultOutput.Statistics.RecordsScanned,
	)
	columnsMap := make(map[string]int)
	rowsMap := make([]map[string]interface{}, 0, len(getQueryResultOutput.Results))
	ignoreFields = append([]string{"@ptr"}, ignoreFields...)
	for _, results := range getQueryResultOutput.Results {
		row := make(map[string]interface{}, len(results))
		for _, result := range results {
			if lo.Contains(ignoreFields, *result.Field) {
				continue
			}
			if _, ok := columnsMap[*result.Field]; !ok {
				columnsMap[*result.Field] = len(columnsMap)
			}
			if result.Value == nil {
				row[*result.Field] = ""
			} else {
				row[*result.Field] = *result.Value
			}
		}
		rowsMap = append(rowsMap, row)
	}
	return queryrunner.NewQueryResultWithRowsMap(name, *params.QueryString, columnsMap, rowsMap), nil
}

func (r *QueryRunner) waitQueryResult(ctx context.Context, queryStart time.Time, params *cloudwatchlogs.GetQueryResultsInput) (*cloudwatchlogs.GetQueryResultsOutput, error) {
	reqID := "-"
	hctx, ok := queryrunner.GetQueryRunningContext(ctx)
	if ok {
		reqID = fmt.Sprintf("%d", hctx.ReqID)
	}
	waiter := &queryrunner.Waiter{
		StartTime: queryStart,
		MinDelay:  100 * time.Microsecond,
		MaxDelay:  5 * time.Second,
		Timeout:   15 * time.Minute,
		Jitter:    200 * time.Millisecond,
	}
	log.Printf("[info][%s] wait finish query", reqID)
	if err := hctx.ChangeSQSMessageVisibilityTimeout(ctx, 30*time.Second); err != nil {
		log.Printf("[warn][%s] failed change sqs message visibility timeout: %v", reqID, err)
	}
	for waiter.Continue(ctx) {
		elapsedTime := time.Since(queryStart)
		log.Printf("[debug][%s] wating cloudwatch logs insights query elapsed_time=%s", reqID, elapsedTime)
		getQueryResultOutput, err := r.client.GetQueryResults(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("get query results:%w", err)
		}
		if elapsedTime > 10*time.Second {
			if err := hctx.ChangeSQSMessageVisibilityTimeout(ctx, 30*time.Second); err != nil {
				log.Printf("[warn][%s] failed change sqs message visibility timeout: %v", reqID, err)
			}
		}

		switch getQueryResultOutput.Status {
		case types.QueryStatusRunning, types.QueryStatusScheduled:
		case types.QueryStatusComplete:
			return getQueryResultOutput, nil
		default:
			return nil, errors.New("get query result unknown status ")
		}
	}
	return nil, errors.New("wait query result timeout")
}
