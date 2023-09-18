package sqlprovider

import (
	"context"
	"database/sql"
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

type QueryParams []QueryParam

func DecodeExpressionToQueryParams(expr hcl.Expression, evalCtx *hcl.EvalContext) (QueryParams, error) {
	var params QueryParams
	if listExpr, d := hcl.ExprList(expr); !d.HasErrors() {
		for i, expr := range listExpr {
			params = append(params, QueryParam{
				Index: i,
				Value: expr,
			})
		}
		return params, nil
	}
	if mapExpr, d := hcl.ExprMap(expr); !d.HasErrors() {
		for _, kv := range mapExpr {
			nameValue, err := kv.Key.Value(evalCtx)
			if err != nil {
				return nil, fmt.Errorf("sqlprovider.DecodeExpressionToQueryParams: Evalute key: %w", err)
			}
			var name string
			if err := hclutil.UnmarshalCTYValue(nameValue, &name); err != nil {
				return nil, fmt.Errorf("sqlprovider.DecodeExpressionToQueryParams UnmarshalCTYValue: %w", err)
			}
			params = append(params, QueryParam{
				Name:  name,
				Value: kv.Value,
			})
		}
		return params, nil
	}
	return nil, fmt.Errorf("sqlprovider.DecodeExpressionToQueryParams: invalid expression type")
}

func (ps QueryParams) ToInterfaceSlice(evalCtx *hcl.EvalContext) ([]interface{}, error) {
	var params []interface{}
	for _, param := range ps {
		value, err := param.Value.Value(evalCtx)
		if err != nil {
			return nil, fmt.Errorf("sqlprovider.QueryParams.ToInterfaceSlice: Evalute param: %w", err)
		}
		var v interface{}
		if err := hclutil.UnmarshalCTYValue(value, &v); err != nil {
			return nil, fmt.Errorf("sqlprovider.QueryParams.ToInterfaceSlice UnmarshalCTYValue: %w", err)
		}
		if param.Name != "" {
			params = append(params, sql.Named(param.Name, v))
			continue
		}
		params = append(params, v)
	}
	return params, nil
}

type Query struct {
	Provider  *Provider
	Name      string
	Params    QueryParams
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
			var err error
			query.Params, err = DecodeExpressionToQueryParams(attr.Expr, evalCtx)
			if err != nil {
				return nil, fmt.Errorf("sqlprovider.NewQuery: %w", err)
			}
		}
	}
	return query, nil
}

func (q *Query) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*prepalert.QueryResult, error) {
	params, err := q.Params.ToInterfaceSlice(evalCtx)
	if err != nil {
		return nil, fmt.Errorf("sqlprovider.Query.Run: %w", err)
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
	qr, err := prepalert.NewQueryResultWithSQLRows(q.Name, statement, params, rows)
	if err != nil {
		return nil, fmt.Errorf("sqlprovider.Query.Run: %w", err)
	}
	return qr, nil
}
