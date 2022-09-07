package queryrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agext/levenshtein"
	"github.com/hashicorp/hcl/v2"
	"github.com/olekukonko/tablewriter"
)

var queryRunners = make(map[string]*QueryRunnerDefinition)

type QueryRunnerDefinition struct {
	TypeName                     string
	RestrictQueryRunnerBlockFunc func(body hcl.Body) hcl.Diagnostics
	RestrictQueryBlockFunc       func(body hcl.Body) hcl.Diagnostics
	BuildQueryRunnerFunc         func(name string, body hcl.Body, ctx *hcl.EvalContext) (QueryRunner, hcl.Diagnostics)
}

type QueryRunner interface {
	Prepare(name string, body hcl.Body, ctx *hcl.EvalContext) (PreparedQuery, hcl.Diagnostics)
}

type PreparedQuery interface {
	Name() string
	Run(ctx context.Context, data interface{}) (*QueryResult, error)
}

func Register(def *QueryRunnerDefinition) error {
	if def == nil {
		return errors.New("QueryRunnerDefinition is nil")
	}
	if def.TypeName == "" {
		return errors.New("TypeName is required")
	}
	if def.RestrictQueryRunnerBlockFunc == nil {
		return errors.New("RestrictQueryRunnerBlockFunc is required")
	}
	if def.RestrictQueryBlockFunc == nil {
		return errors.New("RestrictQueryBlockFunc is required")
	}
	if def.BuildQueryRunnerFunc == nil {
		return errors.New("BuildQueryRunnerFunc is required")
	}
	queryRunners[def.TypeName] = def
	return nil
}

func getQueryRunner(queryRunnerType string, body hcl.Body) (*QueryRunnerDefinition, hcl.Diagnostics) {
	def, ok := queryRunners[queryRunnerType]
	if !ok {
		for suggestion := range queryRunners {
			dist := levenshtein.Distance(queryRunnerType, suggestion, nil)
			if dist < 3 {
				return nil, hcl.Diagnostics([]*hcl.Diagnostic{
					{
						Severity: hcl.DiagError,
						Summary:  "Invalid query_runner type",
						Detail:   fmt.Sprintf(`The query runner type "%s" is invalid. Did you mean "%s"?`, queryRunnerType, suggestion),
						Subject:  body.MissingItemRange().Ptr(),
					},
				})
			}
		}
		return nil, hcl.Diagnostics([]*hcl.Diagnostic{
			{
				Severity: hcl.DiagError,
				Summary:  "Invalid query_runner type",
				Detail:   fmt.Sprintf(`The query runner type "%s" is invalid. maybe not implemented or typo`, queryRunnerType),
				Subject:  body.MissingItemRange().Ptr(),
			},
		})
	}
	return def, nil
}

func RestrictQueryRunnerBlock(queryRunnerType string, body hcl.Body) hcl.Diagnostics {
	def, diags := getQueryRunner(queryRunnerType, body)
	if diags.HasErrors() {
		return diags
	}
	diags = append(diags, def.RestrictQueryRunnerBlockFunc(body)...)
	return diags
}

func RestrictQueryBlock(queryRunnerType string, body hcl.Body) hcl.Diagnostics {
	def, diags := getQueryRunner(queryRunnerType, body)
	if diags.HasErrors() {
		return diags
	}
	diags = append(diags, def.RestrictQueryBlockFunc(body)...)
	return diags
}

func NewQueryRunner(queryRunnerType string, name string, body hcl.Body, ctx *hcl.EvalContext) (QueryRunner, hcl.Diagnostics) {
	def, diags := getQueryRunner(queryRunnerType, body)
	if diags.HasErrors() {
		return nil, diags
	}
	queryRunner, buildDiags := def.BuildQueryRunnerFunc(name, body, ctx)
	diags = append(diags, buildDiags...)
	return queryRunner, diags
}

type QueryResult struct {
	Name    string
	Query   string
	Columns []string
	Rows    [][]string
}

func NewEmptyQueryResult(name string, query string) *QueryResult {
	return &QueryResult{
		Name:    name,
		Query:   query,
		Columns: make([]string, 0),
		Rows:    make([][]string, 0),
	}
}

func NewQueryResult(name string, query string, columns []string, rows [][]string) *QueryResult {
	return &QueryResult{
		Name:    name,
		Query:   query,
		Columns: columns,
		Rows:    rows,
	}
}

func (qr *QueryResult) ToTable() string {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader(qr.Columns)
	table.AppendBulk(qr.Rows)
	table.Render()
	return buf.String()
}

func (qr *QueryResult) ToVertical() string {
	var builder strings.Builder
	for i, row := range qr.Rows {
		fmt.Fprintf(&builder, "********* %d. row *********\n", i+1)
		for j, column := range qr.Columns {
			fmt.Fprintf(&builder, "  %s: %s\n", column, row[j])
		}
	}
	return builder.String()
}
