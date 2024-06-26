package prepalert_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/hcl/v2"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/canyon"
	"github.com/mashiike/canyon/canyontest"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/mock"
	"github.com/mashiike/prepalert/provider"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"go.uber.org/mock/gomock"
)

func TestAppLoadConfig__Simple(t *testing.T) {
	app := LoadApp(t, "testdata/config/simple.hcl")
	require.Equal(t, "prepalert", app.SQSQueueName())
	require.ElementsMatch(t, []string{}, app.ProviderList())
	require.ElementsMatch(t, []string{}, app.QueryList())
	require.False(t, app.EnableBasicAuth())
	rules := app.Rules()
	require.Len(t, rules, 1)
	require.Len(t, rules[0].DependsOnQueries(), 0)

	t.Run("AsServer", func(t *testing.T) {
		var sendMessageCount int
		var sendMessageRequestId string
		h := canyontest.AsServer(
			app,
			canyon.WorkerSenderFunc(func(r *http.Request, _ *canyon.SendOptions) (string, error) {
				sendMessageCount++
				sendMessageRequestId = r.Header.Get(prepalert.HeaderRequestID)
				return "dummy-message-id", nil
			}),
		)
		r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "example_webhook.json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, 1, sendMessageCount)
		require.Equal(t, resp.Header.Get(prepalert.HeaderRequestID), sendMessageRequestId)
	})

	t.Run("AsViewer", func(t *testing.T) {
		h := canyontest.AsServer(
			app,
			canyon.WorkerSenderFunc(func(r *http.Request, _ *canyon.SendOptions) (string, error) {
				t.Error("unexpected call SendMessage")
				t.FailNow()
				return "dummy-message-id", nil
			}),
		)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("AsWorker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
		client := mock.NewMockMackerelClient(ctrl)
		client.EXPECT().GetAlert("2bj...").Return(
			&mackerel.Alert{
				ID:   "2bj...",
				Memo: "this is a pen",
			}, nil,
		).Times(1)
		client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error) {
				require.Equal(t, "2bj...", alertID)
				g.Assert(t, "simple_as_worker__updated_alert_memo", []byte(param.Memo))
				return &mackerel.UpdateAlertResponse{
					Memo: param.Memo,
				}, nil
			},
		).Times(1)
		app.SetMackerelClient(client)
		h := canyontest.AsWorker(app)
		r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "example_webhook.json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAppLoadConfig__WithQuery(t *testing.T) {
	restore := flextime.Fix(time.UnixMilli(1473129912693).Add(5 * time.Minute))
	defer restore()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockProvider := mock.NewMockProvider(ctrl)
	mockServelessProvider := mock.NewMockProvider(ctrl)
	provider.RegisterProvider("redshift_data", func(pp *provider.ProviderParameter) (provider.Provider, error) {
		require.Equal(t, "redshift_data", pp.Type)
		switch pp.Name {
		case "default":
			return mockProvider, nil
		case "serverless":
			return mockServelessProvider, nil
		default:
			return nil, errors.New("unknown provider name")
		}
	})
	t.Cleanup(func() {
		provider.UnregisterProvider("redshift_data")
	})
	mockQuery := mock.NewMockQuery(ctrl)
	mockProvider.EXPECT().NewQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockQuery, nil).Times(1)
	mockServelessProvider.EXPECT().NewQuery(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockQuery, nil).Times(1)

	app := LoadApp(t, "testdata/config/with_query/")
	require.Equal(t, "prepalert", app.SQSQueueName())
	require.ElementsMatch(t, []string{
		"redshift_data.default",
		"redshift_data.serverless",
	}, app.ProviderList())
	require.ElementsMatch(t, []string{
		"query.redshift_data.access_logs",
		"query.redshift_data.serverless_access_logs",
	}, app.QueryList())
	require.True(t, app.EnableBasicAuth())
	require.Equal(t, "interval:10s jitter:30s max_interval:300s backoff_factor:4.00", app.RetryPolicy().String())
	rules := app.Rules()
	require.Len(t, rules, 2)
	require.ElementsMatch(t, []string{
		"query.redshift_data.access_logs",
		"query.redshift_data.serverless_access_logs",
	}, append(rules[0].DependsOnQueries(), rules[1].DependsOnQueries()...))

	t.Run("AsWorker", func(t *testing.T) {
		g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
		mockQuery.EXPECT().Run(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, evalCtx *hcl.EvalContext) (*provider.QueryResult, error) {
				var v json.RawMessage
				hclutil.UnmarshalCTYValue(cty.ObjectVal(evalCtx.Variables), &v)
				actual := strings.ReplaceAll(string(v), prepalert.Version, "<app_versio>")
				g.AssertJson(t, "with_query_as_worker__eval_ctx_variables", actual)
				return provider.NewQueryResultWithJSONLines(
					"access_logs", "select * from access_logs", nil,
					map[string]json.RawMessage{
						"Name":   json.RawMessage(`"A"`),
						"Sign":   json.RawMessage(`"The Good"`),
						"Reason": json.RawMessage(`"The Bad"`),
					},
					map[string]json.RawMessage{
						"Name": json.RawMessage(`"B"`),
						"Sign": json.RawMessage(`"The Ugly"`),
					},
				), nil
			},
		)
		client := mock.NewMockMackerelClient(ctrl)
		client.EXPECT().GetAlert("2bj...").Return(
			&mackerel.Alert{
				ID:   "2bj...",
				Memo: "this is a pen\n\n## Prepalert\n\n### rule.simple\nHow do you respond to alerts?\nDescribe information about your alert response here.\n",
			}, nil,
		).Times(1)
		client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error) {
				require.Equal(t, "2bj...", alertID)
				g.Assert(t, "with_query_as_worker__updated_alert_memo", []byte(param.Memo))
				return &mackerel.UpdateAlertResponse{
					Memo: param.Memo,
				}, nil
			},
		).Times(1)
		client.EXPECT().FindGraphAnnotations(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*mackerel.GraphAnnotation{}, nil).Times(1)
		client.EXPECT().CreateGraphAnnotation(gomock.Any()).DoAndReturn(
			func(param *mackerel.GraphAnnotation) (*mackerel.GraphAnnotation, error) {
				g.AssertJson(t, "with_query_as_worker__created_graph_annotation", param)
				param.ID = "dummy-graph-annotation-id"
				return param, nil
			},
		).Times(1)
		app.SetMackerelClient(client)
		h := canyontest.AsWorker(app)
		r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "example_webhook.json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAppLoadConfig__WithS3Backend(t *testing.T) {
	t.Setenv("TZ", "UTC")
	t.Setenv("GOOGLE_CLIENT_ID", "dummy-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "dummy-client-secret")
	t.Setenv("SESSION_ENCRYPT_KEY", "passpasspasspass")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockS3Client := mock.NewMockS3Client(ctrl)
	prepalert.GlobalS3Client = mockS3Client
	t.Cleanup(func() {
		prepalert.GlobalS3Client = nil
	})
	app := LoadApp(t, "testdata/config/with_s3_backend.hcl")
	require.Equal(t, "prepalert", app.SQSQueueName())
	require.ElementsMatch(t, []string{}, app.ProviderList())
	require.ElementsMatch(t, []string{}, app.QueryList())
	require.False(t, app.EnableBasicAuth())
	backend, ok := app.Backend().(*prepalert.S3Backend)
	require.True(t, ok)
	require.Equal(t, "prepalert-information", backend.BucketName)
	require.Equal(t, "alerts/", *backend.ObjectKeyPrefix)
	require.Equal(t, "http://localhost:8080", backend.ViewerBaseURL.String())
	require.True(t, backend.EnableGoogleAuth())
	rules := app.Rules()
	require.Len(t, rules, 1)

	t.Run("AsWorker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
		mockS3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, param *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
				g.AssertJson(t, "with_s3backend_as_worker__put_object_input", param)
				bs, err := io.ReadAll(param.Body)
				require.NoError(t, err)
				g.Assert(t, "with_s3backend_as_worker__put_object_body", bs)
				return &s3.PutObjectOutput{}, nil
			},
		).Times(1)

		client := mock.NewMockMackerelClient(ctrl)
		client.EXPECT().GetAlert("2bj...").Return(
			&mackerel.Alert{
				ID:        "2bj...",
				MonitorID: "4gx...",
				Memo:      "",
			}, nil,
		).Times(1)
		client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error) {
				require.Equal(t, "2bj...", alertID)
				g.Assert(t, "with_s3backend_as_worker__updated_alert_memo", []byte(param.Memo))
				return &mackerel.UpdateAlertResponse{
					Memo: param.Memo,
				}, nil
			},
		).Times(1)
		app.SetMackerelClient(client)
		h := canyontest.AsWorker(app)
		r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "example_webhook.json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAppLoadConfig__Dynamic(t *testing.T) {
	app := LoadApp(t, "testdata/config/dynamic.hcl")
	require.Equal(t, "prepalert", app.SQSQueueName())
	require.ElementsMatch(t, []string{}, app.ProviderList())
	require.ElementsMatch(t, []string{}, app.QueryList())
	require.False(t, app.EnableBasicAuth())
	rules := app.Rules()
	require.Len(t, rules, 3)
	require.Len(t, rules[0].DependsOnQueries(), 0)
}

func TestAppLoadConfig__Invalid(t *testing.T) {
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
	cases := []struct {
		name string
		path string
	}{
		{"invalid_schema", "testdata/config/invalid_schema/"},
		{"invalid_duplicate", "testdata/config/invalid_duplicate.hcl"},
		{"invalid_provider", "testdata/config/invalid_provider.hcl"},
		{"invalid_version", "testdata/config/invalid_version.hcl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := prepalert.New("dummy-api-key")
			var buf bytes.Buffer
			err := app.LoadConfig(tc.path, func(lco *prepalert.LoadConfigOptions) {
				lco.DiagnosticDestination = &buf
				lco.Color = aws.Bool(false)
				lco.Width = aws.Uint(88)
			})
			require.Error(t, err)
			g.Assert(t, "load_config_diagnotics__"+tc.name, buf.Bytes())
		})
	}
}

func TestAppLoadConfig__When_Is_List(t *testing.T) {
	app := LoadApp(t, "testdata/config/when_is_list.hcl")
	require.Equal(t, "prepalert", app.SQSQueueName())
	require.ElementsMatch(t, []string{}, app.ProviderList())
	require.ElementsMatch(t, []string{}, app.QueryList())
	require.False(t, app.EnableBasicAuth())
	rules := app.Rules()
	require.Len(t, rules, 1)
	require.Len(t, rules[0].DependsOnQueries(), 0)

	t.Run("AsWorker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
		client := mock.NewMockMackerelClient(ctrl)
		client.EXPECT().GetAlert("2bj...").Return(
			&mackerel.Alert{
				ID:        "2bj...",
				MonitorID: "4gx...",
				Memo:      "How do you respond to alerts?",
			}, nil,
		).Times(1)
		client.EXPECT().GetMonitor("4gx...").Return(
			&mackerel.MonitorServiceMetric{
				ID:   "4gx...",
				Name: "test-monitor",
			}, nil,
		)
		client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error) {
				require.Equal(t, "2bj...", alertID)
				g.Assert(t, "when_is_list_as_worker__updated_alert_memo", []byte(param.Memo))
				return &mackerel.UpdateAlertResponse{
					Memo: param.Memo,
				}, nil
			},
		).Times(1)
		app.SetMackerelClient(client)
		h := canyontest.AsWorker(app)
		r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "example_webhook.json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAppLoadConfig__WithPlugin(t *testing.T) {
	app := LoadApp(t, "testdata/config/with_plugin.hcl")
	require.Equal(t, "prepalert", app.SQSQueueName())
	require.ElementsMatch(t, []string{"test.default"}, app.ProviderList())
	require.ElementsMatch(t, []string{"query.test.hoge"}, app.QueryList())
	require.False(t, app.EnableBasicAuth())
	rules := app.Rules()
	require.Len(t, rules, 1)
	require.ElementsMatch(t, []string{"query.test.hoge"}, rules[0].DependsOnQueries())
	t.Run("AsWorker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
		client := mock.NewMockMackerelClient(ctrl)
		client.EXPECT().GetAlert("2bj...").Return(
			&mackerel.Alert{
				ID:   "2bj...",
				Memo: "this is a pen",
			}, nil,
		).Times(1)
		client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error) {
				require.Equal(t, "2bj...", alertID)
				g.Assert(t, "with_plugin_as_worker__updated_alert_memo", []byte(param.Memo))
				return &mackerel.UpdateAlertResponse{
					Memo: param.Memo,
				}, nil
			},
		).Times(1)
		app.SetMackerelClient(client)
		h := canyontest.AsWorker(app)
		r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "example_webhook.json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAppLoadConfig__WithExamplePlugin(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		cw := csv.NewWriter(w)
		cw.Comma = ','
		cw.Write([]string{"id", "name"})
		cw.Write([]string{"1", "hoge"})
		cw.Write([]string{"2", "fuga"})
		cw.Flush()
	}))
	defer s.Close()
	t.Setenv("TEST_SERVER_ENDPOINT", s.URL)
	app := LoadApp(t, "testdata/config/with_example_plugin.hcl")
	require.Equal(t, "prepalert", app.SQSQueueName())
	require.ElementsMatch(t, []string{"http.default"}, app.ProviderList())
	require.ElementsMatch(t, []string{"query.http.test_server"}, app.QueryList())
	require.False(t, app.EnableBasicAuth())
	rules := app.Rules()
	require.Len(t, rules, 1)
	require.ElementsMatch(t, []string{"query.http.test_server"}, rules[0].DependsOnQueries())
	t.Run("AsWorker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
		client := mock.NewMockMackerelClient(ctrl)
		client.EXPECT().GetAlert("2bj...").Return(
			&mackerel.Alert{
				ID:   "2bj...",
				Memo: "this is a pen",
			}, nil,
		).Times(1)
		client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error) {
				require.Equal(t, "2bj...", alertID)
				actual := strings.ReplaceAll(param.Memo, s.URL, "http://<test_server_endpoint>")
				g.Assert(t, "with_example_plugin_as_worker__updated_alert_memo", []byte(actual))
				return &mackerel.UpdateAlertResponse{
					Memo: param.Memo,
				}, nil
			},
		).Times(1)
		app.SetMackerelClient(client)
		h := canyontest.AsWorker(app)
		r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "example_webhook.json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAppLoadConfig__WithRulePriorty(t *testing.T) {
	app := LoadApp(t, "testdata/config/with_rule_priority.hcl")
	rules := app.Rules()
	require.Len(t, rules, 2)
	require.Equal(t, rules[0].Name(), "first")
	require.Equal(t, rules[1].Name(), "second")

	t.Run("AsWorker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
		client := mock.NewMockMackerelClient(ctrl)
		callCount := 0
		lastMemo := "this is a pen"
		client.EXPECT().GetAlert("2bj...").DoAndReturn(
			func(alertID string) (*mackerel.Alert, error) {
				return &mackerel.Alert{
					ID:   "2bj...",
					Memo: lastMemo,
				}, nil
			},
		).Times(1)
		client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
			func(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error) {
				require.Equal(t, "2bj...", alertID)
				callCount++
				g.Assert(t, fmt.Sprintf("with_rule_priority_as_worker__updated_alert_memo_%d", callCount), []byte(param.Memo))
				lastMemo = param.Memo
				return &mackerel.UpdateAlertResponse{
					Memo: param.Memo,
				}, nil
			},
		).Times(1)
		app.SetMackerelClient(client)
		h := canyontest.AsWorker(app)
		r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "example_webhook.json"))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
