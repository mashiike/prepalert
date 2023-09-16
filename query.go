package prepalert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/olekukonko/tablewriter"
	"github.com/zclconf/go-cty/cty"
)

//go:generate mockgen -source=$GOFILE -destination=./mock/mock_$GOFILE -package=mock

type ProviderParameter struct {
	Type   string
	Name   string
	Params map[string]cty.Value
}

func (p *ProviderParameter) String() string {
	return fmt.Sprintf("%s.%s", p.Type, p.Name)
}

type Provider interface {
	NewQuery(name string, body hcl.Body, evalCtx *hcl.EvalContext) (Query, error)
}

type ProviderFactory func(*ProviderParameter) (Provider, error)

type Query interface {
	Run(ctx context.Context, evalCtx *hcl.EvalContext) (*QueryResult, error)
}

type QueryResult struct {
	Name    string              `cty:"name" json:"name"`
	Query   string              `cty:"query" json:"query"`
	Columns []string            `cty:"columns" json:"columns"`
	Rows    [][]json.RawMessage `cty:"rows" json:"rows"`
}

func NewQueryResultWithJSONLines(name string, query string, lines ...map[string]json.RawMessage) *QueryResult {
	qr := &QueryResult{
		Name:  name,
		Query: query,
	}
	columns := make([]string, 0)
	columnsMap := make(map[string]int)
	for _, line := range lines {
		for k := range line {
			if _, ok := columnsMap[k]; !ok {
				columnsMap[k] = len(columns)
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

func NewQueryResult(name string, query string, columns []string, rows [][]json.RawMessage) *QueryResult {
	return &QueryResult{
		Name:    name,
		Query:   query,
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

var (
	providersMu       sync.RWMutex
	providerFactories = map[string]ProviderFactory{}
)

func RegisterProvider(typeName string, factory ProviderFactory) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providerFactories[typeName] = factory
}

func UnregisterProvider(typeName string) {
	providersMu.Lock()
	defer providersMu.Unlock()
	delete(providerFactories, typeName)
}

func NewProvider(param *ProviderParameter) (Provider, error) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	factory, ok := providerFactories[param.Type]
	if !ok {
		return nil, fmt.Errorf("unknown provider type %q", param.Type)
	}
	return factory(param)
}

type ProviderDefinition interface {
	NewProvider(*ProviderParameter) (Provider, error)
}

type ProviderParameters []*ProviderParameter

func (p ProviderParameters) MarshalCTYValue() (cty.Value, error) {
	values := make(map[string]map[string]cty.Value)
	for _, provider := range p {
		if values[provider.Type] == nil {
			values[provider.Type] = make(map[string]cty.Value)
		}
		values[provider.Type][provider.Name] = cty.ObjectVal(provider.Params)
	}
	types := make(map[string]cty.Value)
	for k, v := range values {
		types[k] = cty.ObjectVal(v)
	}
	return cty.ObjectVal(types), nil
}
