package hclconfig

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/prepalert/queryrunner"
	"github.com/mashiike/prepalert/queryrunner/redshiftdata"
	"github.com/stretchr/testify/require"
)

func requireConfigEqual(t *testing.T, cfg1 *Config, cfg2 *Config) {
	t.Helper()
	diff := cmp.Diff(
		cfg1, cfg2,
		cmpopts.IgnoreUnexported(PrepalertBlock{}, redshiftdata.QueryRunner{}, redshiftdata.PreparedQuery{}),
		cmpopts.IgnoreFields(Config{}, "Queries"),
		cmpopts.IgnoreFields(RuleBlock{}, "QueriesExpr", "ParamsExpr", "Queries"),
		cmpopts.IgnoreFields(S3BackendBlock{}, "ObjectKeyTemplate", "ViewerBaseURL", "ViewerSessionEncryptKey"),
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
	os.Setenv("SESSION_ENCRYPT_KEY", "passpasspasspass")
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
								Queries:    make(map[string]queryrunner.PreparedQuery),
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
						Rules: []*RuleBlock{
							{
								Name: "alb_target_5xx",
								Alert: AlertBlock{
									MonitorName: generics.Ptr("ALB Target 5xx"),
								},
								Queries: map[string]queryrunner.PreparedQuery{
									"alb_target_5xx_info": nil,
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
						Rules: []*RuleBlock{
							{
								Name: "alb_target_5xx",
								Alert: AlertBlock{
									MonitorName: generics.Ptr("ALB Target 5xx"),
								},
								Queries: map[string]queryrunner.PreparedQuery{
									"alb_target_5xx_info": nil,
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
			path:     "testdata/s3_backend",
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
								BucketName:                    "prepalert-infomation",
								ObjectKeyPrefix:               generics.Ptr("alerts/"),
								ObjectKeyTemplateString:       generics.Ptr("{{ .Alert.OpenedAt | to_time | strftime `%Y/%m/%d/%H` }}/"),
								ViewerBaseURLString:           "http://localhost:8080",
								ViewerGoogleClientID:          generics.Ptr(""),
								ViewerGoogleClientSecret:      generics.Ptr(""),
								ViewerSessionEncryptKeyString: generics.Ptr("passpasspasspass"),
							},
						},
						Rules: []*RuleBlock{
							{
								Name: "simple",
								Alert: AlertBlock{
									Any: generics.Ptr(true),
								},
								Queries:    make(map[string]queryrunner.PreparedQuery),
								Infomation: "How do you respond to alerts?\nDescribe information about your alert response here.\n",
							},
						},
					}, cfg)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.casename, func(t *testing.T) {
			cfg, err := Load(c.path, "current", func(loader *hclconfig.Loader) {
				loader.DiagnosticWriter(hclconfig.DiagnosticWriterFunc(func(diag *hcl.Diagnostic) error {
					t.Log(diagnosticToString(diag))
					return nil
				}))
			})
			require.NoError(t, err)
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
				"testdata/invalid_required_version_format/config.hcl:2,5-40: Invalid version constraint format; Malformed constraint: invalid format",
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
				"testdata/duplicate/config.hcl:12,1-39: Duplicate query_runner \"redshift_data\" configuration; A redshift_data query_runner named \"default\" was already declared at testdata/duplicate/config.hcl:7,1-39. query_runner names must unique per type in a configuration",
				"testdata/duplicate/config.hcl:25,1-28: Duplicate query declaration; A query named \"alb_target_5xx_info\" was already declared at testdata/duplicate/config.hcl:16,1-28. query names must unique within a configuration",
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
			actual := make([]string, 0, len(c.expected))
			_, err := Load(c.path, "current", func(loader *hclconfig.Loader) {
				loader.DiagnosticWriter(hclconfig.DiagnosticWriterFunc(func(diag *hcl.Diagnostic) error {
					actual = append(actual, diagnosticToString(diag))
					return nil
				}))
			})
			require.Error(t, err)
			require.ElementsMatch(t, c.expected, actual)
		})
	}
}
