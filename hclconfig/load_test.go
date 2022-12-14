package hclconfig

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/queryrunner"
	"github.com/mashiike/queryrunner/redshiftdata"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func requireConfigEqual(t *testing.T, cfg1 *Config, cfg2 *Config) {
	t.Helper()
	diff := cmp.Diff(
		cfg1, cfg2,
		cmpopts.IgnoreUnexported(
			PrepalertBlock{},
			redshiftdata.QueryRunner{},
			redshiftdata.PreparedQuery{},
			hcl.TraverseRoot{},
			hcl.TraverseAttr{},
		),
		cmpopts.IgnoreFields(Config{}, "Queries", "EvalContext"),
		cmpopts.IgnoreFields(RuleBlock{}, "QueriesExpr", "ParamsExpr", "Queries"),
		cmpopts.IgnoreFields(S3BackendBlock{}, "ViewerBaseURL", "ViewerSessionEncryptKey"),
		cmpopts.IgnoreFields(hclsyntax.FunctionCallExpr{}, "NameRange", "OpenParenRange", "CloseParenRange"),
		cmpopts.IgnoreFields(hclsyntax.LiteralValueExpr{}, "SrcRange"),
		cmpopts.IgnoreFields(hclsyntax.TemplateExpr{}, "SrcRange"),
		cmpopts.IgnoreFields(hclsyntax.ScopeTraversalExpr{}, "SrcRange"),
		cmpopts.IgnoreFields(hclsyntax.ObjectConsExpr{}, "SrcRange", "OpenRange"),
		cmpopts.IgnoreFields(hcl.TraverseRoot{}, "SrcRange"),
		cmpopts.IgnoreFields(hcl.TraverseAttr{}, "SrcRange"),
		cmpopts.EquateEmpty(),
		cmp.Comparer(func(x, y cty.Value) bool {
			return x.GoString() == y.GoString()
		}),
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
								Queries: make(map[string]queryrunner.PreparedQuery),
								Information: &hclsyntax.TemplateExpr{
									Parts: []hclsyntax.Expression{
										&hclsyntax.LiteralValueExpr{
											Val: cty.StringVal("How do you respond to alerts?\n"),
										},
										&hclsyntax.LiteralValueExpr{
											Val: cty.StringVal("Describe information about your alert response here.\n"),
										},
									},
								},
								PostGraphAnnotation: true,
								UpdateAlertMemo:     true,
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
								Params: cty.ObjectVal(map[string]cty.Value{
									"hoge":    cty.StringVal("hoge"),
									"version": cty.StringVal("current"),
								}),
								Information: &hclsyntax.TemplateExpr{
									Parts: []hclsyntax.Expression{
										&hclsyntax.LiteralValueExpr{
											Val: cty.StringVal("5xx info:\n"),
										},
										&hclsyntax.ScopeTraversalExpr{
											Traversal: hcl.Traversal{
												hcl.TraverseRoot{Name: "runtime"},
												hcl.TraverseAttr{Name: "query_result"},
												hcl.TraverseAttr{Name: "alb_target_5xx_info"},
												hcl.TraverseAttr{Name: "table"},
											},
										},
										&hclsyntax.LiteralValueExpr{
											Val: cty.StringVal("\n"),
										},
									},
								},
								PostGraphAnnotation: false,
								UpdateAlertMemo:     false,
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
								Information: &hclsyntax.FunctionCallExpr{
									Name: "templatefile",
									Args: []hclsyntax.Expression{
										&hclsyntax.TemplateExpr{
											Parts: []hclsyntax.Expression{
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("./information_template.txt"),
												},
											},
										},
										&hclsyntax.ObjectConsExpr{
											Items: []hclsyntax.ObjectConsItem{
												{
													KeyExpr: &hclsyntax.ObjectConsKeyExpr{
														Wrapped: &hclsyntax.ScopeTraversalExpr{
															Traversal: hcl.Traversal{
																hcl.TraverseRoot{Name: "runtime"},
															},
														},
													},
													ValueExpr: &hclsyntax.ScopeTraversalExpr{
														Traversal: hcl.Traversal{
															hcl.TraverseRoot{Name: "runtime"},
														},
													},
												},
											},
										},
									},
								},
								PostGraphAnnotation: true,
								UpdateAlertMemo:     true,
							},
							{
								Name: "constant",
								Alert: AlertBlock{
									MonitorID: generics.Ptr("xxxxxxxxxxxx"),
								},
								Information: &hclsyntax.TemplateExpr{
									Parts: []hclsyntax.Expression{
										&hclsyntax.LiteralValueExpr{
											Val: cty.StringVal("prepalert: "),
										},
										&hclsyntax.ScopeTraversalExpr{
											Traversal: hcl.Traversal{
												hcl.TraverseRoot{Name: "var"},
												hcl.TraverseAttr{Name: "version"},
											},
										},
									},
								},
								PostGraphAnnotation: true,
								UpdateAlertMemo:     true,
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
								BucketName:      "prepalert-information",
								ObjectKeyPrefix: generics.Ptr("alerts/"),
								ObjectKeyTemplate: generics.Ptr(hcl.Expression(&hclsyntax.FunctionCallExpr{
									Name: "strftime",
									Args: []hclsyntax.Expression{
										&hclsyntax.TemplateExpr{
											Parts: []hclsyntax.Expression{
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("%"),
												},
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("Y/"),
												},
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("%"),
												},
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("m/"),
												},
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("%"),
												},
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("d/"),
												},
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("%"),
												},
												&hclsyntax.LiteralValueExpr{
													Val: cty.StringVal("H/"),
												},
											},
										},
										&hclsyntax.ScopeTraversalExpr{
											Traversal: hcl.Traversal{
												hcl.TraverseRoot{Name: "runtime"},
												hcl.TraverseAttr{Name: "event"},
												hcl.TraverseAttr{Name: "alert"},
												hcl.TraverseAttr{Name: "opened_at"},
											},
										},
									},
								})),
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
								Queries: make(map[string]queryrunner.PreparedQuery),
								Information: &hclsyntax.TemplateExpr{
									Parts: []hclsyntax.Expression{
										&hclsyntax.LiteralValueExpr{
											Val: cty.StringVal("How do you respond to alerts?\n"),
										},
										&hclsyntax.LiteralValueExpr{
											Val: cty.StringVal("Describe information about your alert response here.\n"),
										},
									},
								},
								PostGraphAnnotation:               true,
								UpdateAlertMemo:                   true,
								MaxGraphAnnotationDescriptionSize: generics.Ptr(int(1024)),
								MaxAlertMemoSize:                  generics.Ptr(int(1024)),
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
				"testdata/invalid_schema/config.hcl:6,13-13: Missing required argument; The argument \"information\" is required, but no definition was found.",
				"testdata/invalid_schema/config.hcl:6,13-13: Missing required block; The block \"alert\" is required, but no definition was found. which alerts does this rule respond to?",
				"testdata/invalid_schema/query.hcl:2,14-23: Invalid Query Runner; can not set constant value. please write as runner = \"query_runner.type.name\"",
				"testdata/invalid_schema/query.hcl:6,14-9,6: Invalid Query Runner; can not set multiple query runners. please write as runner = \"query_runner.type.name\"",
			},
		},
		{
			casename: "duplicate blocks",
			path:     "testdata/duplicate",
			expected: []string{
				"testdata/duplicate/config.hcl:40,9-34: Invalid Relation; query.alb_target_5xx_info is not found: rule.queries depends on \"query\" block, please write as \"query.name\"",
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
