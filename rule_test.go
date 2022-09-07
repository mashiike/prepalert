package prepalert_test

import (
	"context"
	"testing"

	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/prepalert/queryrunner"
	"github.com/stretchr/testify/require"
)

func TestRuleRenderMemo(t *testing.T) {
	cases := []struct {
		name          string
		cfg           *hclconfig.RuleBlock
		data          *prepalert.RenderInfomationData
		expectedError bool
		expectedMemo  string
	}{
		{
			name: "empty query event data only",
			cfg: &hclconfig.RuleBlock{
				Alert: hclconfig.AlertBlock{
					MonitorName: generics.Ptr("hoge"),
				},
				Infomation: "{{ .Alert.OpenedAt | to_time | strftime_in_zone `%Y-%m-%d %H:%M:%S` `Asia/Tokyo`}}",
			},
			data: &prepalert.RenderInfomationData{
				WebhookBody:  LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json"),
				QueryResults: make(map[string]*queryrunner.QueryResult),
			},
			expectedMemo: "2016-09-06 11:45:12",
		},
		{
			name: "invalid template",
			cfg: &hclconfig.RuleBlock{
				Alert: hclconfig.AlertBlock{
					MonitorName: generics.Ptr("hoge"),
				},
				Infomation: "{{ .Alert.OpenedAt | to_time | strftime_in_zone `%O%E%Q%1` `Asia/Tokyo`}}",
			},
			data: &prepalert.RenderInfomationData{
				WebhookBody:  LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json"),
				QueryResults: make(map[string]*queryrunner.QueryResult),
			},
			expectedError: true,
		},
		{
			name: "render query_result",
			cfg: &hclconfig.RuleBlock{
				Alert: hclconfig.AlertBlock{
					MonitorName: generics.Ptr("hoge"),
				},
				Infomation: "{{ index .QueryResults `hoge_result` | to_table }}",
			},
			data: &prepalert.RenderInfomationData{
				WebhookBody: LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json"),
				QueryResults: map[string]*queryrunner.QueryResult{
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
					),
				},
			},
			expectedMemo: string(LoadFile(t, "testdata/table.txt")),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rule, err := prepalert.NewRule(nil, c.cfg)
			require.NoError(t, err)
			actual, err := rule.RenderInfomation(context.Background(), c.data)
			if c.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.EqualValues(t, c.expectedMemo, actual)
		})
	}
}
