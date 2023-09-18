package s3select

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/prepalert/provider"
	"github.com/mashiike/prepalert/provider/sqlprovider"
	_ "github.com/mashiike/s3-select-sql-driver"
)

var (
	defaultExpressionExpr hcl.Expression
)

func init() {
	provider.RegisterProvider("s3_select", NewProvider)
	var diags hcl.Diagnostics
	defaultExpressionExpr, diags = hclsyntax.ParseExpression([]byte(`"SELECT * FROM s3object s"`), "expression.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		panic(diags)
	}
}

type Provider struct {
	Type string
	Name string
	ProviderParameter
}

type ProviderParameter struct {
	Region    string `json:"region" hcl:"region"`
	ParseTime bool   `json:"parse_time" hcl:"parse_time"`
}

func NewProvider(pp *provider.ProviderParameter) (*Provider, error) {
	p := &Provider{
		Type: pp.Type,
		Name: pp.Name,
		ProviderParameter: ProviderParameter{
			Region:    os.Getenv("AWS_REGION"),
			ParseTime: true,
		},
	}
	if err := json.Unmarshal(pp.Params, &p.ProviderParameter); err != nil {
		return nil, fmt.Errorf("failed to parse params: %w", err)
	}
	return p, nil
}

type QueryParamter struct {
	Expression         hcl.Expression           `hcl:"expression,optional"`
	Parameters         hcl.Expression           `hcl:"params,optional"`
	BucketName         string                   `hcl:"bucket_name"`
	ObjectKeyPrefix    hcl.Expression           `hcl:"object_key_prefix"`
	CompressionType    *string                  `hcl:"compression_type"`
	Format             *string                  `hcl:"format"`
	InputSerialization *InputSerializationBlock `hcl:"input_serialization,block"`
}

type InputSerializationBlock struct {
	CompressionType *string       `hcl:"compression_type" json:"CompressionType,omitempty"`
	CSV             *CSVBlock     `hcl:"csv,block" json:"CSV,omitempty"`
	JSON            *JSONBlock    `hcl:"json,block" json:"JSON,omitempty"`
	Parquet         *ParquetBlock `hcl:"parquet,block" json:"Parquet,omitempty"`
}

type CSVBlock struct {
	AllowQuotedRecordDelimiter *bool   `hcl:"allow_quoted_record_delimiter" json:"AllowQuotedRecordDelimiter,omitempty"`
	FileHeaderInfo             *string `hcl:"file_header_info" json:"FileHeaderInfo,omitempty"`
	FieldDelimiter             *string `hcl:"field_delimiter" json:"FieldDelimiter,omitempty"`
	QuoteCharacter             *string `hcl:"quote_character" json:"QuoteCharacter,omitempty"`
	QuoteEscapeCharacter       *string `hcl:"quote_escape_character" json:"QuoteEscapeCharacter,omitempty"`
	RecordDelimiter            *string `hcl:"record_delimiter" json:"RecordDelimiter,omitempty"`
	Comments                   *string `hcl:"comments" json:"Comments,omitempty"`
}

type JSONBlock struct {
	Type string `hcl:"type" json:"Type,omitempty"`
}

type ParquetBlock struct{}

type Query struct {
	Name             string
	Provder          *Provider
	Expression       hcl.Expression
	ObjectKeyPrefix  hcl.Expression
	BucketName       string
	ExpressionParams sqlprovider.QueryParams
	DSNQueryParams   url.Values
}

func (p *Provider) NewQuery(name string, body hcl.Body, evalCtx *hcl.EvalContext) (provider.Query, error) {
	var params QueryParamter
	if diags := gohcl.DecodeBody(body, evalCtx, &params); diags.HasErrors() {
		return nil, diags
	}
	if params.Expression == nil {
		params.Expression = defaultExpressionExpr
	}
	query := &Query{
		Name:            name,
		Provder:         p,
		Expression:      params.Expression,
		BucketName:      params.BucketName,
		ObjectKeyPrefix: params.ObjectKeyPrefix,
		DSNQueryParams:  make(url.Values),
	}
	if params.Parameters != nil {
		ps, err := sqlprovider.DecodeExpressionToQueryParams(params.Parameters, evalCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to decode params: %w", err)
		}
		query.ExpressionParams = ps
	}
	if p.ParseTime {
		query.DSNQueryParams.Set("parse_time", "true")
	}
	if p.Region != "" {
		query.DSNQueryParams.Set("region", p.Region)
	}
	if params.CompressionType != nil {
		query.DSNQueryParams.Set("compression_type", strings.ToLower(*params.CompressionType))
	}
	if params.Format != nil {
		query.DSNQueryParams.Set("format", strings.ToLower(*params.Format))
	}
	if params.InputSerialization != nil {
		bs, err := json.Marshal(params.InputSerialization)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input_serialization: %w", err)
		}
		query.DSNQueryParams.Set("input_serialization", base64.URLEncoding.EncodeToString(bs))
	}
	return query, nil
}

func (q *Query) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*provider.QueryResult, error) {
	var objectKeyPrefix, exprssion string
	if err := gohcl.DecodeExpression(q.ObjectKeyPrefix, evalCtx, &objectKeyPrefix); err != nil {
		return nil, fmt.Errorf("failed to decode object_key_prefix: %w", err)
	}
	if err := gohcl.DecodeExpression(q.Expression, evalCtx, &exprssion); err != nil {
		return nil, fmt.Errorf("failed to decode expression: %w", err)
	}
	params, err := q.ExpressionParams.ToInterfaceSlice(evalCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert params: %w", err)
	}
	db, err := sql.Open("s3-select", fmt.Sprintf("s3://%s/%s?%s", q.BucketName, objectKeyPrefix, q.DSNQueryParams.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to open s3-select: %w", err)
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, exprssion, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer rows.Close()
	qr, err := provider.NewQueryResultWithSQLRows(q.Name, exprssion, params, rows)
	if err != nil {
		return nil, fmt.Errorf("failed to create query result: %w", err)
	}
	return qr, nil
}
