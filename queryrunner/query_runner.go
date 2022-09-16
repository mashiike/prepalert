package queryrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/agext/levenshtein"
	"github.com/hashicorp/hcl/v2"
	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
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
	log.Printf("[debug] restrict query_runner with def type  %s", def.TypeName)
	diags = append(diags, def.RestrictQueryRunnerBlockFunc(body)...)
	return diags
}

func RestrictQueryBlock(queryRunnerType string, body hcl.Body) hcl.Diagnostics {
	def, diags := getQueryRunner(queryRunnerType, body)
	if diags.HasErrors() {
		return diags
	}
	log.Printf("[debug] restrict query with def type  %s", def.TypeName)
	diags = append(diags, def.RestrictQueryBlockFunc(body)...)
	return diags
}

func NewQueryRunner(queryRunnerType string, name string, body hcl.Body, ctx *hcl.EvalContext) (QueryRunner, hcl.Diagnostics) {
	def, diags := getQueryRunner(queryRunnerType, body)
	if diags.HasErrors() {
		return nil, diags
	}
	queryRunner, buildDiags := def.BuildQueryRunnerFunc(name, body, ctx)
	log.Printf("[debug] build query_runner `%s` as %T", queryRunnerType, queryRunner)
	diags = append(diags, buildDiags...)
	log.Printf("[debug] build query_runner `%s`, `%d` error diags", queryRunnerType, len(diags.Errs()))
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

func NewQueryResultWithJSONLines(name string, query string, lines [][]byte) *QueryResult {
	columnsMap := make(map[string]int)
	rowsMap := make([]map[string]interface{}, 0, len(lines))
	for _, line := range lines {
		var v map[string]interface{}
		log.Println("[debug] NewQueryResultWithJSONLines:", string(line))
		if err := json.Unmarshal(line, &v); err == nil {
			rowsMap = append(rowsMap, v)
			for columnName := range v {
				if _, ok := columnsMap[columnName]; !ok {
					columnsMap[columnName] = len(columnsMap)
				}
			}
		} else {
			log.Println("[warn] unmarshal err", err)
		}
	}
	return NewQueryResultWithRowsMap(name, query, columnsMap, rowsMap)
}

func NewQueryResultWithRowsMap(name, query string, columnsMap map[string]int, rowsMap []map[string]interface{}) *QueryResult {
	queryResults := &QueryResult{
		Name:  name,
		Query: query,
	}
	columnsEntries := lo.Entries(columnsMap)
	sort.Slice(columnsEntries, func(i, j int) bool {
		return columnsEntries[i].Value < columnsEntries[j].Value
	})
	rows := make([][]string, 0, len(rowsMap))
	for _, rowMap := range rowsMap {
		row := make([]string, 0, len(columnsEntries))
		for _, e := range columnsEntries {
			if v, ok := rowMap[e.Key]; ok {
				row = append(row, fmt.Sprintf("%v", v))
			} else {
				row = append(row, "")
			}
		}
		rows = append(rows, row)
	}
	queryResults.Rows = rows
	queryResults.Columns = lo.Map(columnsEntries, func(e lo.Entry[string, int], _ int) string {
		return e.Key
	})
	return queryResults
}

func (qr *QueryResult) ToTable(optFns ...func(*tablewriter.Table)) string {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader(qr.Columns)
	for _, optFn := range optFns {
		optFn(table)
	}
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

func (qr *QueryResult) ToJSON() string {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	for _, row := range qr.Rows {
		v := make(map[string]string, len(qr.Columns))
		for i, column := range qr.Columns {
			v[column] = row[i]
		}
		encoder.Encode(v)
	}
	return builder.String()
}
