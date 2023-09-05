package prepalert

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/queryrunner"
	"github.com/zclconf/go-cty/cty"
)

type EvalContextBuilder struct {
	Parent  *hcl.EvalContext
	Runtime *RuntimeVariables
}

func (b *EvalContextBuilder) Build() (*hcl.EvalContext, error) {
	var evalCtx *hcl.EvalContext
	if b.Parent != nil {
		evalCtx = b.Parent.NewChild()
	} else {
		evalCtx = &hcl.EvalContext{}
	}
	rv, err := hclutil.MarshalCTYValue(b.Runtime)
	if err != nil {
		return nil, err
	}
	evalCtx.Variables = map[string]cty.Value{
		"var": cty.ObjectVal(map[string]cty.Value{
			"version": cty.StringVal(Version),
		}),
		"runtime": rv,
	}
	return evalCtx, nil
}

type RuntimeVariables struct {
	Params       cty.Value               `cty:"params"`
	Event        *WebhookBody            `cty:"event"`
	QueryResults map[string]*QueryResult `cty:"query_result,omitempty"`
}

type QueryResult queryrunner.QueryResult

func (qr *QueryResult) MarshalCTYValue() (cty.Value, error) {
	return (*queryrunner.QueryResult)(qr).MarshalCTYValue(), nil
}
