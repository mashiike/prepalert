package hclconfig

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/prepalert/queryrunner/redshiftdata"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func requireConfigEqual(t *testing.T, cfg1 *Config, cfg2 *Config) {
	t.Helper()
	diff := cmp.Diff(
		cfg1, cfg2,
		cmpopts.IgnoreUnexported(PrepalertBlock{}, redshiftdata.QueryRunner{}, redshiftdata.PreparedQuery{}),
		cmpopts.IgnoreFields(Config{}, "Queries"),
		cmpopts.IgnoreFields(PrepalertBlock{}, "RequiredVersionExpr"),
		cmpopts.IgnoreFields(RuleBlock{}, "QueriesExpr", "ParamsExpr"),
		cmpopts.IgnoreFields(QueryBlock{}, "RunnerExpr", "Remain"),
		cmpopts.IgnoreFields(QueryRunnerBlock{}, "Remain"),
		cmpopts.IgnoreFields(S3BackendBlock{}, "ObjectKeyTemplate", "ViewerBaseURL", "Remain"),
		cmpopts.EquateEmpty(),
	)
	if diff != "" {
		require.FailNow(t, diff)
	}
}

func diagnosticToString(diag *hcl.Diagnostic) string {
	if diag.Subject == nil {
		return fmt.Sprintf("%s; %s", diag.Summary, diag.Detail)
	}
	return fmt.Sprintf("%s: %s; %s", diag.Subject, diag.Summary, diag.Detail)
}

func TestLoadNoError(t *testing.T) {
	os.Setenv("TEST_CLUSTER", "test")
	os.Setenv("TEST_ENV", "env")
	cases := []struct {
		casename string
		path     string
		check    func(t *testing.T, cfg *Config)
	}{
		{
			casename: "simple config",
			path:     "testdata/simple",
			check: func(t *testing.T, cfg *Config) {
				require.Error(t, cfg.ValidateVersion("v0.0.0"))
				require.NoError(t, cfg.ValidateVersion("v0.2.0"))
				require.Equal(t, 1, len(cfg.Rules))
				requireConfigEqual(t,
					&Config{
						Prepalert: PrepalertBlock{
							SQSQueueName: "prepalert",
							Service:      "prod",
						},
						Rules: []*RuleBlock{
							{
								Name: "simple",
								Alert: AlertBlock{
									Any: generics.Ptr(true),
								},
								Queries:    make(map[string]*QueryBlock),
								Infomation: "How do you respond to alerts?\nDescribe information about your alert response here.\n",
							},
						},
					}, cfg)
			},
		},
		{
			casename: "with query config",
			path:     "testdata/with_query",
			check: func(t *testing.T, cfg *Config) {
				require.Error(t, cfg.ValidateVersion("v0.0.0"))
				require.NoError(t, cfg.ValidateVersion("v0.2.0"))
				requireConfigEqual(t,
					&Config{
						Prepalert: PrepalertBlock{
							SQSQueueName: "prepalert",
							Service:      "prod",
						},
						QueryRunners: []*QueryRunnerBlock{
							{
								Type: "redshift_data",
								Name: "default",
								Impl: &redshiftdata.QueryRunner{
									ClusterIdentifier: generics.Ptr("warehouse"),
									Database:          generics.Ptr("dev"),
									DbUser:            generics.Ptr("admin"),
								},
							},
						},
						Rules: []*RuleBlock{
							{
								Name: "alb_target_5xx",
								Alert: AlertBlock{
									MonitorName: generics.Ptr("ALB Target 5xx"),
								},
								Queries: map[string]*QueryBlock{
									"alb_target_5xx_info": {
										Name: "alb_target_5xx_info",
										Impl: &redshiftdata.PreparedQuery{
											SQL: "SELECT *\nFROM access_logs\nLIMIT 1\n",
										},
									},
								},
								Params: map[string]interface{}{
									"hoge":    "hoge",
									"version": "current",
								},
								Infomation: "5xx info:\n{{ index .QueryResults `alb_target_5xx_info` | to_table }}\n",
							},
						},
					}, cfg)
			},
		},
		{
			casename: "file function",
			path:     "testdata/functions",
			check: func(t *testing.T, cfg *Config) {
				require.Error(t, cfg.ValidateVersion("0.0.0"))
				require.NoError(t, cfg.ValidateVersion("0.2.0"))
				requireConfigEqual(t,
					&Config{
						Prepalert: PrepalertBlock{
							SQSQueueName: "prepalert-current",
							Service:      os.Getenv("TEST_ENV"),
							Auth:         &AuthBlock{},
						},
						QueryRunners: []*QueryRunnerBlock{
							{
								Type: "redshift_data",
								Name: "default",
								Impl: &redshiftdata.QueryRunner{
									ClusterIdentifier: generics.Ptr(os.Getenv("TEST_CLUSTER")),
									Database:          generics.Ptr(os.Getenv("TEST_ENV")),
									DbUser:            generics.Ptr("admin"),
								},
							},
						},
						Rules: []*RuleBlock{
							{
								Name: "alb_target_5xx",
								Alert: AlertBlock{
									MonitorName: generics.Ptr("ALB Target 5xx"),
								},
								Queries: map[string]*QueryBlock{
									"alb_target_5xx_info": {
										Name: "alb_target_5xx_info",
										Impl: &redshiftdata.PreparedQuery{
											SQL: "SELECT\n    path, count(*) as cnt\nFROM access_log\nWHERE access_at\n    BETWEEN 'epoch'::TIMESTAMP + interval '{{ .Alert.OpenedAt }} seconds'\n    AND 'epoch'::TIMESTAMP + interval '{{ .Alert.ClosedAt }} seconds'\nGROUP BY 1\n",
										},
									},
								},
								Infomation: "5xx info:\n{{ index .QueryResults `alb_target_5xx_info` | to_table }}\n",
							},
							{
								Name: "constant",
								Alert: AlertBlock{
									MonitorID: generics.Ptr("xxxxxxxxxxxx"),
								},
								Infomation: "prepalert: current",
							},
						},
					}, cfg)
			},
		},
		{
			casename: "s3 backend config",
			path:     "testdata/s3_Backend",
			check: func(t *testing.T, cfg *Config) {
				require.Error(t, cfg.ValidateVersion("v0.0.0"))
				require.NoError(t, cfg.ValidateVersion("v0.2.0"))
				require.Equal(t, 1, len(cfg.Rules))
				requireConfigEqual(t,
					&Config{
						Prepalert: PrepalertBlock{
							SQSQueueName: "prepalert",
							Service:      "prod",
							S3Backend: &S3BackendBlock{
								BucketName:              "prepalert-infomation",
								ObjectKeyPrefix:         generics.Ptr("alerts/"),
								ObjectKeyTemplateString: generics.Ptr("alerts/{{ .Alert.OpenedAt | to_time | strftime `%Y/%m/%d/%H` }}/{{ .Alert.ID }}.txt"),
								ViewerBaseURLString:     "http://localhost:8080",
							},
						},
						Rules: []*RuleBlock{
							{
								Name: "simple",
								Alert: AlertBlock{
									Any: generics.Ptr(true),
								},
								Queries:    make(map[string]*QueryBlock),
								Infomation: "How do you respond to alerts?\nDescribe information about your alert response here.\n",
							},
						},
					}, cfg)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.casename, func(t *testing.T) {
			cfg, _, diags := load(c.path, "current")
			if diags.HasErrors() {
				for _, diag := range diags {
					t.Log(diagnosticToString(diag))
				}
				require.FailNow(t, "diagnostics should no has error ")
			}
			if c.check != nil {
				c.check(t, cfg)
			}
		})
	}
}

func TestLoadError(t *testing.T) {
	cases := []struct {
		casename string
		path     string
		expected []string
	}{
		{
			casename: "required_version is invalid",
			path:     "testdata/invalid_required_version_format",
			expected: []string{
				"testdata/invalid_required_version_format/config.hcl:2,24-40: Invalid version constraint format; Malformed constraint: invalid format",
			},
		},
		{
			casename: "invalid schema",
			path:     "testdata/invalid_schema",
			expected: []string{
				"testdata/invalid_schema/config.hcl:1,11-11: Missing required argument; The argument \"service\" is required, but no definition was found.",
				"testdata/invalid_schema/variable.hcl:1,1-9: Unsupported block type; Blocks of type \"variable\" are not expected here.",
				"testdata/invalid_schema/config.hcl:1,11-11: Missing required argument; The argument \"sqs_queue_name\" is required, but no definition was found.",
				"testdata/invalid_schema/config.hcl:3,5-22: Unsupported argument; An argument named \"invalid_attribute\" is not expected here.",
				"testdata/invalid_schema/config.hcl:6,13-13: Missing required argument; The argument \"infomation\" is required, but no definition was found.",
				"testdata/invalid_schema/config.hcl:6,13-13: Missing required block; The block \"alert\" is required, but no definition was found. which alerts does this rule respond to?",
				"testdata/invalid_schema/query.hcl:2,14-23: Invalid Query Runner; can not set constant value. please write as runner = \"query_runner.type.name\"",
				"testdata/invalid_schema/query.hcl:6,14-9,6: Invalid Query Runner; can not set multiple query runners. please write as runner = \"query_runner.type.name\"",
			},
		},
		{
			casename: "duplicate blocks",
			path:     "testdata/duplicate",
			expected: []string{
				"testdata/duplicate/config.hcl:12,1-39: Duplicate \"query_runner\" name; A query runner named \"default\" in group \"redshift_data\" was already declared at testdata/duplicate/config.hcl:7,1-39. Query runner names must unique",
				"testdata/duplicate/config.hcl:25,1-28: Duplicate \"query\" name; A query named \"alb_target_5xx_info\" was already declared at testdata/duplicate/config.hcl:16,1-28. Query names must unique",
			},
		},
		{
			casename: "empty_query",
			path:     "testdata/empty_query",
			expected: []string{
				"testdata/empty_query/config.hcl:19,21-21: Invalid SQL template; sql is empty",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.casename, func(t *testing.T) {
			_, _, diags := load(c.path, "current")
			require.True(t, diags.HasErrors())
			require.ElementsMatch(t, c.expected, lo.Map(diags, func(diag *hcl.Diagnostic, _ int) string {
				return diagnosticToString(diag)
			}))
		})
	}
}
