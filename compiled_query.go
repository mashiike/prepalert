package prepalert

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/olekukonko/tablewriter"
)

type CompiledQuery interface {
	Name() string
	Run(context.Context, *QueryData) (*QueryResult, error)
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
