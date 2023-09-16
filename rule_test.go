package prepalert_test

/*
func TestRuleBuildInfomation(t *testing.T) {
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
				body := LoadJSON[*prepalert.WebhookBody](t, "example_webhook.json")
				builder := prepalert.EvalContextBuilder{
					Parent: baseCtx,
					Runtime: &prepalert.RuntimeVariables{
						Event: body,
					},
				}
				ctx, err := builder.Build()
				require.NoError(t, err)
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
				body := LoadJSON[*prepalert.WebhookBody](t, "example_webhook.json")
				builder := prepalert.EvalContextBuilder{
					Parent: baseCtx,
					Runtime: &prepalert.RuntimeVariables{
						Event: body,
					},
				}
				ctx, err := builder.Build()
				require.NoError(t, err)
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
				body := LoadJSON[*prepalert.WebhookBody](t, "example_webhook.json")
				builder := prepalert.EvalContextBuilder{
					Parent: baseCtx,
					Runtime: &prepalert.RuntimeVariables{
						Event: body,
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
				return ctx
			},
			expectedMemo: string(LoadFile(t, "testdata/table.txt")),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			client := mock.NewMockMackerelClient(ctrl)
			svc := prepalert.NewMackerelService(client)
			backend := prepalert.NewDiscardBackend()

			rule, err := prepalert.NewRule(svc, backend, c.cfg, "test")
			require.NoError(t, err)
			actual, err := rule.BuildInfomation(c.newCtx(t))
			if c.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.EqualValues(t, c.expectedMemo, actual)
		})
	}
}
*/
