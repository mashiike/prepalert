package sqlprovider

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert"
	"golang.org/x/exp/slog"
)

type Provider struct {
	DB                      *sql.DB
	StatementAttributeName  string
	ParametersAttributeName string
}

func NewProvider(driverName string, dataSourceName string) (*Provider, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("sqlprovider.NewProvider: %w", err)
	}
	return &Provider{
		DB:                      db,
		StatementAttributeName:  "sql",
		ParametersAttributeName: "params",
	}, nil
}

type QueryParam struct {
	Name  string
	Index int
	Value hcl.Expression
}

type Query struct {
	Provider  *Provider
	Name      string
	Params    []QueryParam
	Statement hcl.Expression
}

func (p *Provider) Close() error {
	return p.DB.Close()
}

func (p *Provider) NewQuery(name string, body hcl.Body, evalCtx *hcl.EvalContext) (prepalert.Query, error) {
	query := &Query{
		Provider: p,
		Name:     name,
	}
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: p.StatementAttributeName, Required: true},
		},
	}
	if p.ParametersAttributeName != "" {
		schema.Attributes = append(schema.Attributes, hcl.AttributeSchema{Name: p.ParametersAttributeName, Required: false})
	}
	content, diags := body.Content(schema)
	if diags.HasErrors() {
		return nil, diags
	}
	for _, attr := range content.Attributes {
		switch attr.Name {
		case p.StatementAttributeName:
			query.Statement = attr.Expr
		case p.ParametersAttributeName:
			if listExpr, d := hcl.ExprList(attr.Expr); !d.HasErrors() {
				for i, expr := range listExpr {
					query.Params = append(query.Params, QueryParam{
						Index: i,
						Value: expr,
					})
				}
				continue
			}
			if mapExpr, d := hcl.ExprMap(attr.Expr); !d.HasErrors() {
				for _, kv := range mapExpr {
					nameValue, err := kv.Key.Value(evalCtx)
					if err != nil {
						return nil, fmt.Errorf("sqlprovider.NewQuery: Evalute key: %w", err)
					}
					var name string
					if err := hclutil.UnmarshalCTYValue(nameValue, &name); err != nil {
						return nil, fmt.Errorf("sqlprovider.NewQuery UnmarshalCTYValue: %w", err)
					}
					query.Params = append(query.Params, QueryParam{
						Name:  name,
						Value: kv.Value,
					})
				}
				continue
			}
		}
	}
	return query, nil
}

func (q *Query) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*prepalert.QueryResult, error) {
	var params []interface{}
	for _, param := range q.Params {
		value, err := param.Value.Value(evalCtx)
		if err != nil {
			return nil, fmt.Errorf("sqlprovider.Query.Run: Evalute param: %w", err)
		}
		var v interface{}
		if err := hclutil.UnmarshalCTYValue(value, &v); err != nil {
			return nil, fmt.Errorf("sqlprovider.Query.Run UnmarshalCTYValue: %w", err)
		}
		if param.Name != "" {
			params = append(params, sql.Named(param.Name, v))
			continue
		}
		params = append(params, v)
	}
	return q.RunWithParamters(ctx, evalCtx, params)
}

func (q *Query) RunWithParamters(ctx context.Context, evalCtx *hcl.EvalContext, params []interface{}) (*prepalert.QueryResult, error) {
	value, diags := q.Statement.Value(evalCtx)
	if diags.HasErrors() {
		return nil, fmt.Errorf("sqlprovider.Query.Run: Evalute statement: %w", diags)
	}
	var statement string
	if err := hclutil.UnmarshalCTYValue(value, &statement); err != nil {
		return nil, fmt.Errorf("sqlprovider.Query.Run UnmarshalCTYValue: %w", err)
	}
	slog.InfoContext(ctx, "sqlprovider.Query.Run", "statement", statement, "params", params)
	rows, err := q.Provider.DB.QueryContext(ctx, statement, params...)
	if err != nil {
		return nil, fmt.Errorf("sqlprovider.Query.Run: %w", err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("sqlprovider.Query.Run: %w", err)
	}
	result := &prepalert.QueryResult{
		Name:    q.Name,
		Query:   statement,
		Params:  params,
		Columns: columns,
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		for i := range values {
			values[i] = new(interface{})
		}
		if err := rows.Scan(values...); err != nil {
			return nil, fmt.Errorf("sqlprovider.Query.Run: %w", err)
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
				return nil, fmt.Errorf("sqlprovider.Query.Run: %w", err)
			}
			row[i] = b
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}
