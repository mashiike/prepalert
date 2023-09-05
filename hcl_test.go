package prepalert_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/queryrunner"
	"github.com/stretchr/testify/require"
)

func TestEvalContextBuilder(t *testing.T) {
	builder := prepalert.EvalContextBuilder{
		Parent: hclutil.NewEvalContext(),
		Runtime: &prepalert.RuntimeVariables{
			Event: LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json"),
			QueryResults: map[string]*prepalert.QueryResult{
				"hoge_result": (*prepalert.QueryResult)(queryrunner.NewQueryResult(
					"hoge_result",
					"dummy",
					[]string{"Name", "Sign", "Rating"},
					[][]string{
						{"A", "The Good", "500"},
						{"B", "The Very very Bad Man", "288"},
						{"C", "The Ugly", "120"},
						{"D", "The Gopher", "800"},
					},
				)),
			},
		},
	}
	ctx, err := builder.Build()
	require.NoError(t, err)
	expr, _ := hclsyntax.ParseExpression([]byte("jsonencode(runtime)"), "hoge", hcl.Pos{Line: 1, Column: 1})
	v, _ := expr.Value(ctx)
	require.JSONEq(t, string(LoadFile(t, "testdata/runtime_eval_context.json")), v.AsString())
}
