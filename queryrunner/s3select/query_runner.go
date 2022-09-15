package s3select

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mashiike/prepalert/internal/funcs"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/prepalert/queryrunner"
	"github.com/samber/lo"
)

func init() {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName:                     "s3_select",
		RestrictQueryRunnerBlockFunc: RestrictQueryRunnerBlock,
		RestrictQueryBlockFunc:       RestrictQueryBlock,
		BuildQueryRunnerFunc:         BuildQueryRunner,
	})
	if err != nil {
		panic(fmt.Errorf("register s3_select query runner:%w", err))
	}
	log.Println("[info] load s3_select query runner")
}

func RestrictQueryRunnerBlock(body hcl.Body) hcl.Diagnostics {
	log.Println("[debug] start s3_select query_runner block restriction, on", body.MissingItemRange().String())
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "region",
			},
		},
	}
	_, diags := body.Content(schema)
	log.Printf("[debug] end s3_select query_runner block %d error diags", len(diags.Errs()))
	return diags
}

func RestrictQueryBlock(body hcl.Body) hcl.Diagnostics {
	log.Println("[debug] start s3_select query block restriction, on", body.MissingItemRange().String())
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "expression",
				Required: true,
			},
			{
				Name:     "bucket_name",
				Required: true,
			},
			{
				Name:     "object_key_prefix",
				Required: true,
			},
			{
				Name: "object_key_suffix",
			},
			{
				Name: "scan_limit",
			},
			{
				Name:     "compression_type",
				Required: true,
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "csv",
			},
			{
				Type: "json",
			},
			{
				Type: "parquet",
			},
		},
	}
	content, diags := body.Content(schema)
	log.Printf("[debug] end s3_select query block %d error diags", len(diags.Errs()))
	if len(content.Blocks) == 0 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Require input serialization",
			Detail:   "Input serialization are required: csv, json or parquet block must be inserted.",
			Subject:  body.MissingItemRange().Ptr(),
		})
	}
	if len(content.Blocks) > 1 {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid input serialization",
			Detail:   "Only one csv, json or parquet block can be defined",
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

	Region *string `hcl:"region"`
}

type PreparedQuery struct {
	name   string
	runner *QueryRunner

	Expression      string  `hcl:"expression"`
	BucketName      string  `hcl:"bucket_name"`
	ObjectKeyPrefix string  `hcl:"object_key_prefix"`
	ObjectKeySuffix *string `hcl:"object_key_suffix"`
	ScanLimit       *string `hcl:"scan_limit"`
	CompressionType string  `hcl:"compression_type"`

	CSVBlock     *QueryCSVBlock     `hcl:"csv,block"`
	JSONBlock    *QueryJSONBlock    `hcl:"json,block"`
	ParquetBlock *QueryParquetBlock `hcl:"parquet,block"`

	expressionTemplate      *template.Template
	objectKeyPrefixTemplate *template.Template
	inputSerialization      *types.InputSerialization
	scanLimit               uint64
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
	if q.Expression == "" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid Expression template",
			Detail:   "expression is empty",
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	expressionTemplate, err := template.New(name + "_expression").Funcs(funcs.QueryTemplateFuncMap).Parse(q.Expression)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid Expression template",
			Detail:   fmt.Sprintf("parse expression as go template: %v", err),
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	q.expressionTemplate = expressionTemplate

	objectKeyPrefixTemplate, err := template.New(name + "_object_key_prefix").Funcs(funcs.QueryTemplateFuncMap).Parse(q.ObjectKeyPrefix)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid ObjectKeyPrefix template",
			Detail:   fmt.Sprintf("parse object_key_prefix as go template: %v", err),
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	q.objectKeyPrefixTemplate = objectKeyPrefixTemplate
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
	if q.CSVBlock != nil {
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
		q.inputSerialization.Parquet = &types.ParquetInput{}
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
}

func (q *PreparedQuery) Run(ctx context.Context, data interface{}) (*queryrunner.QueryResult, error) {
	var expressionBuf, objectKeyPrefixBuf bytes.Buffer
	if err := q.expressionTemplate.Execute(&expressionBuf, data); err != nil {
		return nil, fmt.Errorf("execute expression template:%w", err)
	}
	if err := q.objectKeyPrefixTemplate.Execute(&objectKeyPrefixBuf, data); err != nil {
		return nil, fmt.Errorf("execute object_key_prefix template:%w", err)
	}
	params := &runQueryParameters{
		name:               "prepalert-" + q.name,
		expression:         expressionBuf.String(),
		bucket:             q.BucketName,
		objectKeyPrefix:    objectKeyPrefixBuf.String(),
		objectKeySuffix:    *q.ObjectKeySuffix,
		inputSerialization: q.inputSerialization,
		scanLimitation:     q.scanLimit,
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
				return nil, fmt.Errorf("select object: %s : %w", *content.Key, err)
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
	for event := range stream.Events() {
		v, ok := event.(*types.SelectObjectContentEventStreamMemberRecords)
		if ok {
			decoder := json.NewDecoder(bytes.NewReader(v.Value.Payload))
			var err error
			for {
				var v json.RawMessage
				err = decoder.Decode(&v)
				if err != nil {
					if err == io.EOF {
						break
					}
					return nil, err
				}
				lines = append(lines, v)
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
