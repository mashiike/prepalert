package prepalert_test

import (
	"testing"

	"github.com/mashiike/prepalert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoadNoError(t *testing.T) {
	cases := []struct {
		casename string
		path     string
		check    func(t *testing.T, cfg *prepalert.Config)
	}{
		{
			casename: "default config",
			path:     "testdata/default.yaml",
			check: func(t *testing.T, cfg *prepalert.Config) {
				require.EqualValues(t, &prepalert.AuthConfig{
					ClientID:     "hoge",
					ClientSecret: "hoge",
				}, cfg.Auth)
				require.EqualValues(t, prepalert.QueryRunnerConfigs{
					{
						Name:              "default",
						Type:              prepalert.QueryRunnerTypeRedshiftData,
						ClusterIdentifier: "warehouse",
						Database:          "dev",
						DBUser:            "warehouse",
					},
				}, cfg.QueryRunners)
				require.EqualValues(t, []*prepalert.RuleConfig{
					{
						Monitor: &prepalert.MonitorConfig{
							ID: "xxxxxxxxxxx",
						},
						Queries: []*prepalert.QueryConfig{
							{
								Name:   "access_data",
								Runner: "default",
								File:   "./queries/get_access_data.sql",
								Query:  "SELECT\n    path, count(*) as cnt\nFROM access_log\nWHERE access_at\n    BETWEEN 'epoch'::TIMESTAMP + interval '{{ .Alert.OpenedAt }} seconds'\n    AND 'epoch'::TIMESTAMP + interval '{{ .Alert.ClosedAt }} seconds'\nGROUP BY 1\n",
							},
						},
						Memo: &prepalert.MemoConfig{
							File: "./memo/xxxxxxxxxxx.txt",
							Text: "access_log info\n{{ index .QueryResults `access_data` | to_table }}\n",
						},
					},
				}, cfg.Rules)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.casename, func(t *testing.T) {
			cfg := prepalert.DefaultConfig()
			err := cfg.Load(c.path)
			require.NoError(t, err)
			if c.check != nil {
				c.check(t, cfg)
			}
		})
	}
}
