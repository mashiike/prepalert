package provider

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
)

type QueryResult struct {
	Name    string              `cty:"name" json:"name"`
	Query   string              `cty:"query" json:"query"`
	Params  []interface{}       `cty:"params" json:"params,omitempty"`
	Columns []string            `cty:"columns" json:"columns"`
	Rows    [][]json.RawMessage `cty:"rows" json:"rows"`
}

func NewQueryResultWithSQLRows(name string, query string, params []interface{}, rows *sql.Rows) (*QueryResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("NewQueryResultWithSQLRows: Columns: %w", err)
	}
	result := &QueryResult{
		Name:    name,
		Query:   query,
		Params:  params,
		Columns: columns,
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		for i := range values {
			values[i] = new(interface{})
		}
		if err := rows.Scan(values...); err != nil {
			return nil, fmt.Errorf("NewQueryResultWithSQLRows: Scan: %w", err)
		}
		row := make([]json.RawMessage, len(columns))
		for i := range columns {
			val := *(values[i].(*interface{}))
			if val == nil {
				row[i] = json.RawMessage("null")
				continue
			}
			b, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("NewQueryResultWithSQLRows: Marshal: %w", err)
			}
			row[i] = b
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

func NewQueryResultWithJSONLines(name string, query string, params []interface{}, lines ...map[string]json.RawMessage) *QueryResult {
	qr := &QueryResult{
		Name:   name,
		Query:  query,
		Params: params,
	}
	columns := make([]string, 0)
	columnsMap := make(map[string]int)
	for i, line := range lines {
		for k := range line {
			if _, ok := columnsMap[k]; !ok {
				columnsMap[k] = i
				columns = append(columns, k)
			}
		}
	}
	sort.SliceStable(columns, func(i, j int) bool {
		if strings.HasPrefix(columns[i], "time") {
			if !strings.HasPrefix(columns[j], "time") {
				return true
			}
		}
		if columnsMap[columns[i]] > columnsMap[columns[j]] {
			return false
		}
		return columns[i] > columns[j]
	})
	columnsIndex := make(map[string]int)
	for i, c := range columns {
		columnsIndex[c] = i
	}
	rows := make([][]json.RawMessage, 0)
	for _, line := range lines {
		row := make([]json.RawMessage, len(columns))
		for k, v := range line {
			if i, ok := columnsIndex[k]; ok {
				row[i] = v
			}
		}
		rows = append(rows, row)
	}
	qr.Columns = columns
	qr.Rows = rows
	return qr
}

func NewQueryResult(name string, query string, params []interface{}, columns []string, rows [][]json.RawMessage) *QueryResult {
	return &QueryResult{
		Name:    name,
		Query:   query,
		Params:  params,
		Columns: columns,
		Rows:    rows,
	}
}

func (qr *QueryResult) ToTable(optFns ...func(*tablewriter.Table)) string {
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)
	table.SetHeader(qr.Columns)
	for _, optFn := range optFns {
		optFn(table)
	}
	rows := make([][]string, len(qr.Rows))
	for i, row := range qr.Rows {
		rows[i] = make([]string, len(row))
		for j, column := range row {
			var columnStr string
			if err := json.Unmarshal(column, &columnStr); err != nil {
				columnStr = string(column)
			}
			rows[i][j] = columnStr
		}
	}
	table.AppendBulk(rows)
	table.Render()
	return buf.String()
}

func (qr *QueryResult) ToVertical() string {
	var builder strings.Builder
	for i, row := range qr.Rows {
		fmt.Fprintf(&builder, "********* %d. row *********\n", i+1)
		for j, column := range qr.Columns {
			var columnStr string
			if err := json.Unmarshal(row[j], &columnStr); err != nil {
				columnStr = string(column)
			}
			fmt.Fprintf(&builder, "  %s: %s\n", column, columnStr)
		}
	}
	return builder.String()
}

func (qr *QueryResult) ToBorderlessTable() string {
	return qr.ToTable(
		func(table *tablewriter.Table) {
			table.SetCenterSeparator(" ")
			table.SetAutoFormatHeaders(false)
			table.SetAutoWrapText(false)
			table.SetBorder(false)
			table.SetColumnSeparator(" ")
		},
	)
}

func (qr *QueryResult) ToMarkdownTable() string {
	return qr.ToTable(
		func(table *tablewriter.Table) {
			table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			table.SetCenterSeparator("|")
			table.SetAutoFormatHeaders(false)
			table.SetAutoWrapText(false)
		},
	)
}

func (qr *QueryResult) ToJSONLines() string {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	for _, row := range qr.Rows {
		line := make(map[string]json.RawMessage)
		for i, column := range qr.Columns {
			if i >= len(row) {
				continue
			}
			line[column] = row[i]
		}
		encoder.Encode(line)
	}
	return builder.String()
}
