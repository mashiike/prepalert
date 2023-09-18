package provider

import (
	"errors"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclutil"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type EvalContextQueryVariables struct {
	FQN    string       `cty:"fqn"`
	Status string       `cty:"status"`
	Error  string       `cty:"error"`
	Result *QueryResult `cty:"result"`
}

var queryResultCTYType = cty.Object(map[string]cty.Type{
	"name":    cty.String,
	"query":   cty.String,
	"params":  cty.List(cty.DynamicPseudoType),
	"columns": cty.List(cty.String),
	"rows":    cty.List(cty.List(cty.DynamicPseudoType)),
})

func newConvertFunctionForQueryResult(
	description string,
	f func(qr *QueryResult) (string, error),
) function.Function {
	return function.New(&function.Spec{
		Description: description,
		Params: []function.Parameter{
			{
				Name: "query",
				Type: cty.Object(map[string]cty.Type{
					"status": cty.String,
					"result": queryResultCTYType,
				}),
			},
		},
		VarParam: nil,
		Type:     function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			var query EvalContextQueryVariables
			if err := hclutil.UnmarshalCTYValue(args[0], &query); err != nil {
				return cty.UnknownVal(cty.String), fmt.Errorf("failed unmarshal query: %w", err)
			}
			switch query.Status {
			case "success":
				str, err := f(query.Result)
				if err != nil {
					return cty.UnknownVal(cty.String), fmt.Errorf("failed convert query result: %w", err)
				}
				return cty.StringVal(str), nil
			case "failed":
				return cty.StringVal(fmt.Sprintf("[query %q failed: %s]", query.FQN, query.Error)), nil
			case "running", "":
				return cty.StringVal(fmt.Sprintf("[query %q running]", query.FQN)), nil
			}
			return cty.UnknownVal(cty.String), errors.New("query.status unknown")
		},
	})
}

func WithFunctions(evalCtx *hcl.EvalContext) *hcl.EvalContext {
	child := evalCtx.NewChild()
	child.Functions = map[string]function.Function{
		"result_to_table": newConvertFunctionForQueryResult(
			"convert query_result to table format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToTable(), nil
			},
		),
		"result_to_jsonlines": newConvertFunctionForQueryResult(
			"convert query_result to jsonlines format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToJSONLines(), nil
			},
		),
		"result_to_vertical": newConvertFunctionForQueryResult(
			"convert query_result to vertical table format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToVertical(), nil
			},
		),
		"result_to_markdown": newConvertFunctionForQueryResult(
			"convert query_result to markdown table format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToMarkdownTable(), nil
			},
		),
		"result_to_borderless": newConvertFunctionForQueryResult(
			"convert query_result to borderless table format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToBorderlessTable(), nil
			},
		),
	}
	return child
}

func WithQury(evalCtx *hcl.EvalContext, variables *EvalContextQueryVariables) (*hcl.EvalContext, error) {
	if variables == nil {
		return evalCtx, errors.New("variables is nil")
	}
	if variables.FQN == "" {
		return evalCtx, errors.New("variables.fqn is empty")
	}
	switch variables.Status {
	case "success", "failed", "running", "":
	default:
		return evalCtx, fmt.Errorf("unknown query status %q allows only success, failed, running", variables.Status)
	}
	val := map[string]cty.Value{
		"status": cty.StringVal(variables.Status),
		"fqn":    cty.StringVal(variables.FQN),
	}
	if variables.Error != "" {
		val["error"] = cty.StringVal(variables.Error)
	}
	if variables.Result != nil {
		qr, err := hclutil.MarshalCTYValue(variables.Result)
		if err != nil {
			return evalCtx, fmt.Errorf("failed to marshal query result: %w", err)
		}
		val["result"] = qr
	}
	evalCtx = hclutil.WithValue(evalCtx, variables.FQN, cty.ObjectVal(val))
	return evalCtx, nil
}
