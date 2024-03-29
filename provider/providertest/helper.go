package providertest

import (
	"context"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert/provider"
)

// NewQuery is a helper function to create a new query from HCL body and variables.
// this function for testing.
func NewQuery(p provider.Provider, queryName string, hclBody []byte, variables map[string]interface{}) (provider.Query, error) {
	file, diags := hclsyntax.ParseConfig(hclBody, "temporary.hcl", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, diags
	}
	evalCtx := hclutil.NewEvalContext()
	if variables != nil {
		v, err := hclutil.MarshalCTYValue(variables)
		if err != nil {
			return nil, err
		}
		evalCtx.Variables = v.AsValueMap()
	}
	q, err := p.NewQuery(queryName, file.Body, evalCtx)
	if err != nil {
		return nil, err
	}
	return q, nil
}

func RunQuery(ctx context.Context, q provider.Query, variables map[string]interface{}) (*provider.QueryResult, error) {
	evalCtx := hclutil.NewEvalContext()
	if variables != nil {
		v, err := hclutil.MarshalCTYValue(variables)
		if err != nil {
			return nil, err
		}
		evalCtx.Variables = v.AsValueMap()
	}
	return q.Run(ctx, evalCtx)
}
