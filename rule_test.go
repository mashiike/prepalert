package prepalert_test

import (
	"context"
	"testing"

	"github.com/mashiike/prepalert"
	"github.com/stretchr/testify/require"
)

func TestRuleRenderMemo(t *testing.T) {
	cases := []struct {
		name          string
		runners       prepalert.QueryRunners
		cfg           *prepalert.RuleConfig
		data          *prepalert.RenderMemoData
		expectedError bool
		expectedMemo  string
	}{
		{
			name: "empty query event data only",
			cfg: &prepalert.RuleConfig{
				Monitor: &prepalert.MonitorConfig{
					Name: "hoge",
				},
				Queries: make([]*prepalert.QueryConfig, 0),
				Memo: &prepalert.MemoConfig{
					Text: "{{ .Alert.OpenedAt | to_time | strftime_in_zone `%Y-%m-%d %H:%M:%S` `Asia/Tokyo`}}",
				},
			},
			data: &prepalert.RenderMemoData{
				WebhookBody:  LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json"),
				QueryResults: make(map[string]*prepalert.QueryResult),
			},
			expectedMemo: "2016-09-06 11:45:12",
		},
		{
			name: "invalid template",
			cfg: &prepalert.RuleConfig{
				Monitor: &prepalert.MonitorConfig{
					Name: "hoge",
				},
				Queries: make([]*prepalert.QueryConfig, 0),
				Memo: &prepalert.MemoConfig{
					Text: "{{ .Alert.OpenedAt | to_time | strftime_in_zone `%O%E%Q%1` `Asia/Tokyo`}}",
				},
			},
			data: &prepalert.RenderMemoData{
				WebhookBody:  LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json"),
				QueryResults: make(map[string]*prepalert.QueryResult),
			},
			expectedError: true,
		},
		{
			name: "render query_result",
			cfg: &prepalert.RuleConfig{
				Monitor: &prepalert.MonitorConfig{
					Name: "hoge",
				},
				Queries: make([]*prepalert.QueryConfig, 0),
				Memo: &prepalert.MemoConfig{
					Text: "{{ index .QueryResults `hoge_result` | to_table }}",
				},
			},
			data: &prepalert.RenderMemoData{
				WebhookBody: LoadJSON[*prepalert.WebhookBody](t, "testdata/event.json"),
				QueryResults: map[string]*prepalert.QueryResult{
					"hoge_result": prepalert.NewQueryResult(
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
			rule, err := prepalert.NewRule(nil, c.cfg, c.runners)
			require.NoError(t, err)
			actual, err := rule.RenderMemo(context.Background(), c.data)
			if c.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.EqualValues(t, c.expectedMemo, actual)
		})
	}
}
