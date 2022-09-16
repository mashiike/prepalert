package cloudwatchlogsinsights

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mashiike/prepalert/internal/funcs"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/prepalert/queryrunner"
)

func init() {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName:                     "cloudwatch_logs_insights",
		RestrictQueryRunnerBlockFunc: RestrictQueryRunnerBlock,
		RestrictQueryBlockFunc:       RestrictQueryBlock,
		BuildQueryRunnerFunc:         BuildQueryRunner,
	})
	if err != nil {
		panic(fmt.Errorf("register cloudwatch_logs_insights query runner:%w", err))
	}
	log.Println("[info] load cloudwatch_logs_insights query runner")
}

func RestrictQueryRunnerBlock(body hcl.Body) hcl.Diagnostics {
	log.Println("[debug] start cloudwatch_logs_insights query_runner block restriction, on", body.MissingItemRange().String())
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "region",
			},
		},
	}
	_, diags := body.Content(schema)
	log.Printf("[debug] end cloudwatch_logs_insights query_runner block %d error diags", len(diags.Errs()))
	return diags
}

func RestrictQueryBlock(body hcl.Body) hcl.Diagnostics {
	log.Println("[debug] start cloudwatch_logs_insights query block restriction, on", body.MissingItemRange().String())
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "start_time",
			},
			{
				Name: "end_time",
			},
			{
				Name:     "query",
				Required: true,
			},
			{
				Name: "limit",
			},
			{
				Name: "log_group_name",
			},
			{
				Name: "log_group_names",
			},
		},
	}

	content, diags := body.Content(schema)
	log.Printf("[debug] end cloudwatch_logs_insights query block %d error diags", len(diags.Errs()))
	var logGroupNameRange, logGroupNamesRange *hcl.Range
	for _, attr := range content.Attributes {
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
	return diags
}

func BuildQueryRunner(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
	queryRunner := &QueryRunner{}
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

type QueryRunner struct {
	client *cloudwatchlogs.Client

	Region *string `hcl:"region"`
}

type PreparedQuery struct {
	name   string
	runner *QueryRunner

	StartTime   *string `hcl:"start_time"`
	EndTime     *string `hcl:"end_time"`
	QueryString string  `hcl:"query"`
	Limit       *int32  `hcl:"limit"`

	LogGroupName  *string  `hcl:"log_group_name"`
	LogGroupNames []string `hcl:"log_group_names"`

	queryTemplate     *template.Template
	startTimeTemplate *template.Template
	endTimeTemplate   *template.Template
}

type QueryCSVBlock struct {
	AllowQuotedRecordDelimiter *bool   `hcl:"allow_quoted_record_delimiter"`
	FileHeaderInfo             *string `hcl:"file_header_info"`
	FieldDelimiter             *string `hcl:"field_delimiter"`
	QuoteCharacter             *string `hcl:"quote_character"`
	QuoteEscapeCharacter       *string `hcl:"quote_escape_character"`
	RecordDelimiter            *string `hcl:"record_delimiter"`
}

type QueryJSONBlock struct {
	Type string `hcl:"type"`
}

type QueryParquetBlock struct{}

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
	if q.QueryString == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "query is empty",
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	var err error
	q.queryTemplate, err = template.New(name + "query").Funcs(funcs.QueryTemplateFuncMap).Parse(q.QueryString)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   fmt.Sprintf("parse query as go template: %v", err),
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	if q.StartTime == nil || *q.StartTime == "" {
		q.StartTime = generics.Ptr(`{{ .Alert.OpenedAt | to_time | strftime_in_zone "%Y-%m-%dT%H:%M:%S%z" "UTC"  }}`)
	}
	q.startTimeTemplate, err = template.New(name + "start_time").Funcs(funcs.QueryTemplateFuncMap).Parse(*q.StartTime)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid start_time template",
			Detail:   fmt.Sprintf("parse start_time as go template: %v", err),
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	if q.EndTime == nil || *q.EndTime == "" {
		q.EndTime = generics.Ptr(`{{ .Alert.ClosedAt | to_time | strftime_in_zone "%Y-%m-%dT%H:%M:%S%z" "UTC"  }}`)
	}
	q.endTimeTemplate, err = template.New(name + "end_time").Funcs(funcs.QueryTemplateFuncMap).Parse(*q.EndTime)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid start_time template",
			Detail:   fmt.Sprintf("parse start_time as go template: %v", err),
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	return q, diags
}

func (q *PreparedQuery) Name() string {
	return q.name
}

const layout = "2006-01-02T15:04:05-0700"

func (q *PreparedQuery) Run(ctx context.Context, data interface{}) (*queryrunner.QueryResult, error) {
	var queryBuf, startTimeBuf, endTimeBuf bytes.Buffer
	if err := q.queryTemplate.Execute(&queryBuf, data); err != nil {
		return nil, fmt.Errorf("execute query template:%w", err)
	}
	if err := q.startTimeTemplate.Execute(&startTimeBuf, data); err != nil {
		return nil, fmt.Errorf("execute start_time template:%w", err)
	}
	startTime, err := time.Parse(layout, startTimeBuf.String())
	if err != nil {
		return nil, fmt.Errorf("parse start_time: %w", err)
	}
	if err := q.endTimeTemplate.Execute(&endTimeBuf, data); err != nil {
		return nil, fmt.Errorf("execute end_time template:%w", err)
	}
	endTime, err := time.Parse(layout, endTimeBuf.String())
	if err != nil {
		return nil, fmt.Errorf("parse end_time: %w", err)
	}

	params := &cloudwatchlogs.StartQueryInput{
		StartTime:     aws.Int64(startTime.Unix()),
		EndTime:       aws.Int64(endTime.Unix()),
		QueryString:   aws.String(queryBuf.String()),
		Limit:         q.Limit,
		LogGroupName:  q.LogGroupName,
		LogGroupNames: q.LogGroupNames,
	}
	return q.runner.RunQuery(ctx, q.name, params)
}

func (r *QueryRunner) RunQuery(ctx context.Context, name string, params *cloudwatchlogs.StartQueryInput) (*queryrunner.QueryResult, error) {
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
	for _, results := range getQueryResultOutput.Results {
		row := make(map[string]interface{}, len(results))
		for _, result := range results {
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
