package prepalert_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	libhclconfig "github.com/mashiike/hclconfig"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/queryrunner"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestRuleRenderMemo(t *testing.T) {
	baseCtx := libhclconfig.NewEvalContext("testdata")
	baseCtx.Variables = map[string]cty.Value{
		"runtime": cty.UnknownVal(cty.DynamicPseudoType),
	}
	cases := []struct {
		name          string
		cfg           *hclconfig.RuleBlock
		newCtx        func(t *testing.T) *hcl.EvalContext
		expectedError bool
		expectedMemo  string
	}{
		{
			name: "empty query event data only",
			cfg: &hclconfig.RuleBlock{
				Alert: hclconfig.AlertBlock{
					MonitorName: generics.Ptr("hoge"),
				},
				Information: ParseExpression(t, `"${strftime_in_zone("%Y-%m-%d %H:%M:%S","Asia/Tokyo",runtime.event.alert.opened_at)}"`),
			},
			newCtx: func(t *testing.T) *hcl.EvalContext {
				ctx := baseCtx.NewChild()
				body := LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json")
				ctx.Variables = map[string]cty.Value{
					"runtime": cty.ObjectVal(map[string]cty.Value{
						"event":        cty.ObjectVal(body.MarshalCTYValues()),
						"query_reuslt": cty.ObjectVal(map[string]cty.Value{}),
					}),
				}
				return ctx
			},
			expectedMemo: "2016-09-06 11:45:12",
		},
		{
			name: "invalid template",
			cfg: &hclconfig.RuleBlock{
				Alert: hclconfig.AlertBlock{
					MonitorName: generics.Ptr("hoge"),
				},
				Information: ParseExpression(t, `"${strftime_in_zone("%O%E%Q%1","Asia/Tokyo",runtime.event.alert.opened_at)}"`),
			},
			newCtx: func(t *testing.T) *hcl.EvalContext {
				ctx := baseCtx.NewChild()
				body := LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json")
				ctx.Variables = map[string]cty.Value{
					"runtime": cty.ObjectVal(map[string]cty.Value{
						"event":        cty.ObjectVal(body.MarshalCTYValues()),
						"query_reuslt": cty.ObjectVal(map[string]cty.Value{}),
					}),
				}
				return ctx
			},
			expectedError: true,
		},
		{
			name: "render query_result",
			cfg: &hclconfig.RuleBlock{
				Alert: hclconfig.AlertBlock{
					MonitorName: generics.Ptr("hoge"),
				},
				Information: ParseExpression(t, `"${runtime.query_result.hoge_result.table}"`),
			},
			newCtx: func(t *testing.T) *hcl.EvalContext {
				body := LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json")
				ctx := baseCtx.NewChild()
				ctx.Variables = map[string]cty.Value{
					"runtime": cty.ObjectVal(map[string]cty.Value{
						"event": cty.ObjectVal(body.MarshalCTYValues()),
						"query_result": cty.ObjectVal(map[string]cty.Value{
							"hoge_result": queryrunner.NewQueryResult(
								"hoge_result",
								"dummy",
								[]string{"Name", "Sign", "Rating"},
								[][]string{
									{"A", "The Good", "500"},
									{"B", "The Very very Bad Man", "288"},
									{"C", "The Ugly", "120"},
									{"D", "The Gopher", "800"},
								},
							).MarshalCTYValue(),
						}),
					}),
				}
				return ctx
			},
			expectedMemo: string(LoadFile(t, "testdata/table.txt")),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rule, err := prepalert.NewRule(nil, c.cfg)
			require.NoError(t, err)
			actual, err := rule.RenderInformation(c.newCtx(t))
			if c.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.EqualValues(t, c.expectedMemo, actual)
		})
	}
}
