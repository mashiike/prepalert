package s3select

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/prepalert/queryrunner"
	"github.com/samber/lo"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/sync/errgroup"
)

const TypeName = "s3_select"

func init() {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName:             TypeName,
		BuildQueryRunnerFunc: BuildQueryRunner,
	})
	if err != nil {
		panic(fmt.Errorf("register s3_select query runner:%w", err))
	}
	log.Println("[info] load s3_select query runner")
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
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "initialize aws client",
			Detail:   fmt.Sprintf("failed load aws default config:%v", err),
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	queryRunner.client = s3.NewFromConfig(awsCfg)
	return queryRunner, diags
}

type QueryRunner struct {
	client *s3.Client
	name   string

	Region *string `hcl:"region"`
}

func (r *QueryRunner) Name() string {
	return r.name
}

func (r *QueryRunner) Type() string {
	return TypeName
}

type PreparedQuery struct {
	name   string
	runner *QueryRunner

	Expression      hcl.Expression `hcl:"expression"`
	BucketName      string         `hcl:"bucket_name"`
	ObjectKeyPrefix hcl.Expression `hcl:"object_key_prefix"`
	ObjectKeySuffix *string        `hcl:"object_key_suffix"`
	ScanLimit       *string        `hcl:"scan_limit"`
	CompressionType string         `hcl:"compression_type"`
	ContinueOnError bool           `hcl:"continue_on_error,optional"`

	CSVBlock     *QueryCSVBlock     `hcl:"csv,block"`
	JSONBlock    *QueryJSONBlock    `hcl:"json,block"`
	ParquetBlock *QueryParquetBlock `hcl:"parquet,block"`

	inputSerialization *types.InputSerialization
	scanLimit          uint64
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
	log.Printf("[debug] prepare `%s` with s3_select query_runner", name)
	q := &PreparedQuery{
		name:   name,
		runner: r,
	}
	diags := gohcl.DecodeBody(body, ctx, q)
	if diags.HasErrors() {
		return nil, diags
	}
	var err error
	if q.ScanLimit == nil {
		q.ScanLimit = generics.Ptr("1GB")
	}

	q.scanLimit, err = humanize.ParseBytes(*q.ScanLimit)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid scan_limit",
			Detail:   err.Error(),
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}

	if q.ObjectKeySuffix == nil {
		q.ObjectKeySuffix = generics.Ptr("")
	}

	var compressionType types.CompressionType
	compressionTypes := compressionType.Values()
	for _, t := range compressionTypes {
		if strings.EqualFold(q.CompressionType, string(t)) {
			compressionType = t
			break
		}
	}
	if compressionType == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid compression_type",
			Detail: fmt.Sprintf(
				"Must be %s or %s",
				strings.Join(lo.Map(compressionTypes[:len(compressionTypes)-1], func(t types.CompressionType, _ int) string {
					return string(t)
				}), ","),
				compressionTypes[len(compressionTypes)-1],
			),
			Subject: body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}

	q.inputSerialization = &types.InputSerialization{
		CompressionType: compressionType,
	}

	blockCount := 0
	if q.CSVBlock != nil {
		blockCount++
		if q.CSVBlock.AllowQuotedRecordDelimiter == nil {
			q.CSVBlock.AllowQuotedRecordDelimiter = generics.Ptr(false)
		}
		var fileHeaderInfo types.FileHeaderInfo
		if q.CSVBlock.FileHeaderInfo == nil {
			fileHeaderInfo = types.FileHeaderInfoNone
		} else {
			fileHeaderInfoList := fileHeaderInfo.Values()
			for _, t := range fileHeaderInfoList {
				if strings.EqualFold(*q.CSVBlock.FileHeaderInfo, string(t)) {
					fileHeaderInfo = t
					break
				}
			}
			if fileHeaderInfo == "" {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid json.type",
					Detail: fmt.Sprintf(
						"Must be %s or %s",
						strings.Join(lo.Map(fileHeaderInfoList[:len(fileHeaderInfoList)-1], func(t types.FileHeaderInfo, _ int) string {
							return string(t)
						}), ","),
						fileHeaderInfoList[len(fileHeaderInfoList)-1],
					),
					Subject: body.MissingItemRange().Ptr(),
				})
				return nil, diags
			}
		}
		q.inputSerialization.CSV = &types.CSVInput{
			AllowQuotedRecordDelimiter: *q.CSVBlock.AllowQuotedRecordDelimiter,
			FileHeaderInfo:             fileHeaderInfo,
			FieldDelimiter:             q.CSVBlock.FieldDelimiter,
			RecordDelimiter:            q.CSVBlock.RecordDelimiter,
			QuoteCharacter:             q.CSVBlock.QuoteCharacter,
			QuoteEscapeCharacter:       q.CSVBlock.QuoteEscapeCharacter,
		}
	}
	if q.JSONBlock != nil {
		blockCount++
		var jsonType types.JSONType
		jsonTypes := jsonType.Values()
		for _, t := range jsonTypes {
			if strings.EqualFold(q.JSONBlock.Type, string(t)) {
				jsonType = t
				break
			}
		}
		if jsonType == "" {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid json.type",
				Detail: fmt.Sprintf(
					"Must be %s or %s",
					strings.Join(lo.Map(jsonTypes[:len(jsonTypes)-1], func(t types.JSONType, _ int) string {
						return string(t)
					}), ","),
					jsonTypes[len(jsonTypes)-1],
				),
				Subject: body.MissingItemRange().Ptr(),
			})
			return nil, diags
		}
		q.inputSerialization.JSON = &types.JSONInput{
			Type: jsonType,
		}
	}
	if q.ParquetBlock != nil {
		blockCount++
		q.inputSerialization.Parquet = &types.ParquetInput{}
	}
	if blockCount == 0 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Require input serialization",
			Detail:   "Input serialization are required: csv, json or parquet block must be inserted.",
			Subject:  body.MissingItemRange().Ptr(),
		})
	}
	if blockCount > 1 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid input serialization",
			Detail:   "Only one csv, json or parquet block can be defined",
			Subject:  body.MissingItemRange().Ptr(),
		})
	}
	return q, diags
}

func (q *PreparedQuery) Name() string {
	return q.name
}

type runQueryParameters struct {
	name               string
	expression         string
	bucket             string
	objectKeyPrefix    string
	objectKeySuffix    string
	inputSerialization *types.InputSerialization
	scanLimitation     uint64
	continueOnError    bool
}

func (q *PreparedQuery) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*queryrunner.QueryResult, error) {
	expressionValue, diags := q.Expression.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !expressionValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid expression template",
			Detail:   "expression is unknown",
			Subject:  q.Expression.Range().Ptr(),
		})
		return nil, diags
	}
	if expressionValue.Type() != cty.String {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid query template",
			Detail:   "expression is not string",
			Subject:  q.Expression.Range().Ptr(),
		})
		return nil, diags
	}
	expr := expressionValue.AsString()
	if expr == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid expression template",
			Detail:   "expression is empty",
			Subject:  q.Expression.Range().Ptr(),
		})
		return nil, diags
	}

	objectKeyPrefixValue, diags := q.ObjectKeyPrefix.Value(evalCtx)
	if diags.HasErrors() {
		return nil, diags
	}
	if !objectKeyPrefixValue.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid object_key_prefix template",
			Detail:   "object_key_prefix is unknown",
			Subject:  q.ObjectKeyPrefix.Range().Ptr(),
		})
		return nil, diags
	}
	if objectKeyPrefixValue.Type() != cty.String {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid object_key_prefix template",
			Detail:   "object_key_prefix is not string",
			Subject:  q.ObjectKeyPrefix.Range().Ptr(),
		})
		return nil, diags
	}
	objectKeyPrefix := objectKeyPrefixValue.AsString()

	params := &runQueryParameters{
		name:               "prepalert-" + q.name,
		expression:         expr,
		bucket:             q.BucketName,
		objectKeyPrefix:    objectKeyPrefix,
		objectKeySuffix:    *q.ObjectKeySuffix,
		inputSerialization: q.inputSerialization,
		scanLimitation:     q.scanLimit,
		continueOnError:    q.ContinueOnError,
	}
	return q.runner.RunQuery(ctx, params)
}

func (r *QueryRunner) RunQuery(ctx context.Context, params *runQueryParameters) (*queryrunner.QueryResult, error) {
	reqID := "-"
	hctx, ok := queryrunner.GetQueryRunningContext(ctx)
	if ok {
		reqID = fmt.Sprintf("%d", hctx.ReqID)
	}
	log.Printf("[info][%s] start s3 select expression `%s`", reqID, params.name)
	log.Printf("[info][%s] location: s3://%s/%s*%s", reqID, params.bucket, params.objectKeyPrefix, params.objectKeySuffix)
	log.Printf("[debug][%s] original expression: %s", reqID, params.expression)
	expression := strings.ReplaceAll(params.expression, "\n", " ")
	log.Printf("[debug][%s] rewirte expression: %s", reqID, expression)
	p := s3.NewListObjectsV2Paginator(r.client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(params.bucket),
		Prefix:    aws.String(params.objectKeyPrefix),
		Delimiter: aws.String("/"),
	})
	totalScanSize := uint64(0)
	jsonLines := make([][]byte, 0)
	apiCallCount := 0
	if err := hctx.ChangeSQSMessageVisibilityTimeout(ctx, 30*time.Second); err != nil {
		log.Println("[warn] failed change sqs message visibility timeout:", err)
	}
	for p.HasMorePages() {
		if totalScanSize > params.scanLimitation {
			break
		}
		listOutput, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects v2: %w", err)
		}
		for _, content := range listOutput.Contents {
			if totalScanSize > params.scanLimitation {
				log.Printf("[warn][%s] scan limitation exceeded: %s", reqID, humanize.Bytes(uint64(totalScanSize)))
				break
			}
			if params.objectKeySuffix != "" && !strings.HasSuffix(*content.Key, params.objectKeySuffix) {
				continue
			}
			if err := hctx.ChangeSQSMessageVisibilityTimeout(ctx, 30*time.Second); err != nil {
				log.Println("[warn] failed change sqs message visibility timeout:", err)
			}
			log.Printf("[debug][%s] start select object: s3://%s/%s (%s)", reqID, params.bucket, *content.Key, humanize.Bytes(uint64(content.Size)))
			lines, err := r.selectObject(ctx, params.bucket, *content.Key, expression, params.inputSerialization)
			totalScanSize += uint64(content.Size)
			apiCallCount++
			if err != nil {
				if params.continueOnError {
					log.Printf("[warn][%s] select object failed: %v", reqID, err)
				} else {
					return nil, fmt.Errorf("select object: %s : %w", *content.Key, err)
				}
			}
			jsonLines = append(jsonLines, lines...)
			log.Printf("[debug][%s] total scan size: %s, total lines: %d, total object count: %d", reqID, humanize.Bytes(totalScanSize), len(jsonLines), apiCallCount)
		}
	}
	log.Printf("[info][%s] total scan size: %s, total lines: %d, total object count: %d", reqID, humanize.Bytes(totalScanSize), len(jsonLines), apiCallCount)

	return queryrunner.NewQueryResultWithJSONLines(params.name, params.expression, jsonLines), nil
}

func (r *QueryRunner) selectObject(ctx context.Context, bucket string, key string, expression string, inputSerialization *types.InputSerialization) ([][]byte, error) {
	selectOutput, err := r.client.SelectObjectContent(ctx, &s3.SelectObjectContentInput{
		Bucket:             aws.String(bucket),
		Key:                aws.String(key),
		Expression:         aws.String(expression),
		ExpressionType:     types.ExpressionTypeSql,
		InputSerialization: inputSerialization,
		OutputSerialization: &types.OutputSerialization{
			JSON: &types.JSONOutput{
				RecordDelimiter: aws.String("\n"),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	stream := selectOutput.GetStream()
	defer stream.Close()

	lines := make([][]byte, 0)
	pr, pw := io.Pipe()

	eg, egctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer pw.Close()
		for event := range stream.Events() {
			select {
			case <-egctx.Done():
				return nil
			default:
				record, ok := event.(*types.SelectObjectContentEventStreamMemberRecords)
				if ok {
					pw.Write(record.Value.Payload)
				}
			}
		}
		return nil
	})

	decoder := json.NewDecoder(pr)
	for decoder.More() {
		var v json.RawMessage
		if err := decoder.Decode(&v); err != nil {
			return nil, err
		}
		lines = append(lines, v)
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return lines, nil
}
