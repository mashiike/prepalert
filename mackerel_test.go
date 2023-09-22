package prepalert_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/mock"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"go.uber.org/mock/gomock"
)

func ptr[V any](v V) *V {
	return &v
}

func TestMackerelService__UpdateAlertMemo__NowAlertMemoEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("xxxxxxxxxxxxx").Return(
		&mackerel.Alert{
			ID:        "xxxxxxxxxxxxx",
			Memo:      "",
			OpenedAt:  1691323531,
			ClosedAt:  1691323831,
			Status:    "OK",
			Type:      "host",
			MonitorID: "yyyyyyyyyyy",
			HostID:    "zzzzzzzzzzz",
			Value:     0.5,
		},
		nil,
	).Times(1)
	client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(id string, param mackerel.UpdateAlertParam) (mackerel.UpdateAlertResponse, error) {
			require.Equal(t, "xxxxxxxxxxxxx", id)
			t.Log(param.Memo)
			require.Equal(t, "## rule.hoge\nhoge", param.Memo)
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)

	svc := prepalert.NewMackerelService(client)
	err := svc.UpdateAlertMemo(context.Background(), "xxxxxxxxxxxxx", "rule.hoge", "hoge")
	require.NoError(t, err)
}

func TestMackerelService__UpdateAlertMemo__NowAlertMemoEmptyButLarge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	original := prepalert.AlertMemoMaxSize
	t.Cleanup(func() {
		prepalert.AlertMemoMaxSize = original
	})
	prepalert.AlertMemoMaxSize = 100

	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("xxxxxxxxxxxxx").Return(
		&mackerel.Alert{
			ID:        "xxxxxxxxxxxxx",
			Memo:      "",
			OpenedAt:  1691323531,
			ClosedAt:  1691323831,
			Status:    "OK",
			Type:      "host",
			MonitorID: "yyyyyyyyyyy",
			HostID:    "zzzzzzzzzzz",
			Value:     0.5,
		},
		nil,
	).Times(1)
	client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(id string, param mackerel.UpdateAlertParam) (mackerel.UpdateAlertResponse, error) {
			require.Equal(t, "xxxxxxxxxxxxx", id)
			t.Log(param.Memo)
			require.Equal(t, "## rule.hoge\n"+strings.Repeat("A", 84)+"...", param.Memo)
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)

	svc := prepalert.NewMackerelService(client)
	err := svc.UpdateAlertMemo(context.Background(), "xxxxxxxxxxxxx", "rule.hoge", strings.Repeat("A", 100))
	require.NoError(t, err)
}

func TestMackerelService__PostGraphAnnotation__CreateNewAnnotation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	param := &mackerel.GraphAnnotation{
		Title:       "hoge",
		Description: "fuga",
		From:        time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC).Unix(),
		To:          time.Date(2023, 1, 1, 23, 0, 0, 0, time.UTC).Unix(),
		Service:     "piyo",
	}
	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().
		FindGraphAnnotations("piyo", param.From-prepalert.FindGraphAnnotationOffset, param.To+prepalert.FindGraphAnnotationOffset).
		Return(
			[]mackerel.GraphAnnotation{},
			nil,
		).
		Times(1)

	client.EXPECT().
		CreateGraphAnnotation(param).
		Return(
			&mackerel.GraphAnnotation{
				ID: "tora",
			},
			nil,
		)

	svc := prepalert.NewMackerelService(client)
	err := svc.PostGraphAnnotation(context.Background(), param)
	require.NoError(t, err)
}

func TestMackerelService__UpdateAlertMemo__NowAlertMemoNoSectionAndLarge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	original := prepalert.AlertMemoMaxSize
	t.Cleanup(func() {
		prepalert.AlertMemoMaxSize = original
	})
	prepalert.AlertMemoMaxSize = 100
	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("xxxxxxxxxxxxx").Return(
		&mackerel.Alert{
			ID:        "xxxxxxxxxxxxx",
			Memo:      strings.Repeat("A", 85),
			OpenedAt:  1691323531,
			ClosedAt:  1691323831,
			Status:    "OK",
			Type:      "host",
			MonitorID: "yyyyyyyyyyy",
			HostID:    "zzzzzzzzzzz",
			Value:     0.5,
		},
		nil,
	).Times(1)
	client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(id string, param mackerel.UpdateAlertParam) (mackerel.UpdateAlertResponse, error) {
			require.Equal(t, "xxxxxxxxxxxxx", id)
			t.Log(param.Memo)
			require.Equal(t, strings.Repeat("A", 85), param.Memo)
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)

	svc := prepalert.NewMackerelService(client)
	err := svc.UpdateAlertMemo(context.Background(), "xxxxxxxxxxxxx", "rule.hoge", "hoge")
	require.NoError(t, err)
}

func TestMackerelService__UpdateAlertMemo__NowAlertMemoNoSection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("xxxxxxxxxxxxx").Return(
		&mackerel.Alert{
			ID:        "xxxxxxxxxxxxx",
			Memo:      "ABCDEFG",
			OpenedAt:  1691323531,
			ClosedAt:  1691323831,
			Status:    "OK",
			Type:      "host",
			MonitorID: "yyyyyyyyyyy",
			HostID:    "zzzzzzzzzzz",
			Value:     0.5,
		},
		nil,
	).Times(1)
	client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(id string, param mackerel.UpdateAlertParam) (mackerel.UpdateAlertResponse, error) {
			require.Equal(t, "xxxxxxxxxxxxx", id)
			t.Log(param.Memo)
			require.Equal(t, "ABCDEFG\n\n## rule.hoge\nhoge", param.Memo)
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)

	svc := prepalert.NewMackerelService(client)
	err := svc.UpdateAlertMemo(context.Background(), "xxxxxxxxxxxxx", "rule.hoge", "hoge")
	require.NoError(t, err)
}

func TestMackerelService__UpdateAlertMemo__Replace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("xxxxxxxxxxxxx").Return(
		&mackerel.Alert{
			ID:        "xxxxxxxxxxxxx",
			Memo:      "ABCDEFG\n\n## rule.hoge\nhoge\n\n## rule.fuga\nfuga",
			OpenedAt:  1691323531,
			ClosedAt:  1691323831,
			Status:    "OK",
			Type:      "host",
			MonitorID: "yyyyyyyyyyy",
			HostID:    "zzzzzzzzzzz",
			Value:     0.5,
		},
		nil,
	).Times(1)
	client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(id string, param mackerel.UpdateAlertParam) (mackerel.UpdateAlertResponse, error) {
			require.Equal(t, "xxxxxxxxxxxxx", id)
			t.Log(param.Memo)
			require.Equal(t, "ABCDEFG\n\n## rule.hoge\nABCDEFGHIGKLMN\n\n## rule.fuga\nfuga", param.Memo)
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)

	svc := prepalert.NewMackerelService(client)
	err := svc.UpdateAlertMemo(context.Background(), "xxxxxxxxxxxxx", "rule.hoge", "ABCDEFGHIGKLMN")
	require.NoError(t, err)
}

func TestMackerelService__UpdateAlertMemo__ReplaceLarge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	original := prepalert.AlertMemoMaxSize
	t.Cleanup(func() {
		prepalert.AlertMemoMaxSize = original
	})
	prepalert.AlertMemoMaxSize = 100
	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("xxxxxxxxxxxxx").Return(
		&mackerel.Alert{
			ID:        "xxxxxxxxxxxxx",
			Memo:      "ABCDEFG\n\n## rule.hoge\nhoge\n\n## rule.fuga\nfuga",
			OpenedAt:  1691323531,
			ClosedAt:  1691323831,
			Status:    "OK",
			Type:      "host",
			MonitorID: "yyyyyyyyyyy",
			HostID:    "zzzzzzzzzzz",
			Value:     0.5,
		},
		nil,
	).Times(1)
	client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(id string, param mackerel.UpdateAlertParam) (mackerel.UpdateAlertResponse, error) {
			require.Equal(t, "xxxxxxxxxxxxx", id)
			t.Log(param.Memo)
			require.Equal(t, "ABCDEFG\n\n## rule.hoge\n"+strings.Repeat("A", 56)+"...\n\n## rule.fuga\nfuga", param.Memo)
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)

	svc := prepalert.NewMackerelService(client)
	err := svc.UpdateAlertMemo(context.Background(), "xxxxxxxxxxxxx", "rule.hoge", strings.Repeat("A", 100))
	require.NoError(t, err)
}

func TestMackerelService__PostGraphAnnotation__UpdateAnnotationDescription(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	param := &mackerel.GraphAnnotation{
		ID:          "tora",
		Title:       "hoge",
		Description: "fuga",
		From:        time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC).Unix(),
		To:          time.Date(2023, 1, 1, 23, 0, 0, 0, time.UTC).Unix(),
		Service:     "piyo",
	}
	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().
		FindGraphAnnotations("piyo", param.From-prepalert.FindGraphAnnotationOffset, param.To+prepalert.FindGraphAnnotationOffset).
		Return(
			[]mackerel.GraphAnnotation{
				{
					ID:          "tora",
					Title:       "hoge",
					Description: "",
					From:        param.From,
					To:          param.To,
					Service:     param.Service,
				},
			},
			nil,
		).
		Times(1)

	client.EXPECT().
		UpdateGraphAnnotation("tora", &mackerel.GraphAnnotation{
			ID:          "tora",
			Title:       "hoge",
			Description: "fuga",
			From:        param.From,
			To:          param.To,
			Service:     param.Service,
		}).
		Return(
			&mackerel.GraphAnnotation{
				ID: "tora",
			},
			nil,
		)

	svc := prepalert.NewMackerelService(client)
	err := svc.PostGraphAnnotation(context.Background(), param)
	require.NoError(t, err)
}

func TestMackerelService__NewEmulatedWebhookBody__HostMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetOrg().Return(&mackerel.Org{Name: "test-org"}, nil).Times(1)
	client.EXPECT().GetAlert("xxxxxxxxxxx").Return(&mackerel.Alert{
		ID:        "xxxxxxxxxxx",
		OpenedAt:  1691323531,
		ClosedAt:  1691323831,
		Status:    "OK",
		Type:      "host",
		MonitorID: "yyyyyyyyyyy",
		HostID:    "zzzzzzzzzzz",
		Value:     0.5,
	}, nil).Times(1)
	client.EXPECT().GetMonitor("yyyyyyyyyyy").Return(&mackerel.MonitorHostMetric{
		ID:       "yyyyyyyyyyy",
		Name:     "test-monitor",
		Metric:   "custom.host.metric",
		Operator: ">",
		Warning:  ptr(float64(5)),
		Duration: 3,
	}, nil).Times(1)
	client.EXPECT().FindHost("zzzzzzzzzzz").Return(&mackerel.Host{
		ID:        "zzzzzzzzzzz",
		Name:      "test-instance",
		IsRetired: false,
		Memo:      "",
		Status:    "working",
		Roles: mackerel.Roles{
			"prod": []string{"Instance"},
		},
	}, nil).Times(1)

	svc := prepalert.NewMackerelService(client)
	actual, err := svc.NewEmulatedWebhookBody(context.Background(), "xxxxxxxxxxx")
	require.NoError(t, err)
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
	g.AssertJson(t, "mkr_svc__emulated_webhook_body", actual)
}

func TestMackerelService_NewExampleWebhookBody(t *testing.T) {
	svc := prepalert.NewMackerelService(nil)
	actual := svc.NewExampleWebhookBody()
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
	g.AssertJson(t, "mkr_svc__example_webhook_body", actual)
}

func TestWebnookBody__MarshalCTYValues(t *testing.T) {
	body := LoadJSON[prepalert.WebhookBody](t, "example_webhook.json")
	expr, diags := hclsyntax.ParseExpression([]byte("jsonencode(test_event)"), "test.hcl", hcl.InitialPos)
	require.False(t, diags.HasErrors())
	ctx := hclutil.NewEvalContext()
	ctx = ctx.NewChild()
	testEvent, err := hclutil.MarshalCTYValue(body)
	require.NoError(t, err)
	ctx.Variables = map[string]cty.Value{
		"test_event": testEvent,
	}
	value, diags := expr.Value(ctx)
	require.False(t, diags.HasErrors())
	actual := value.AsString()
	t.Log(actual)
	expected := `{
		"alert": {
		  "closed_at": 1473130092,
		  "created_at": 1473129912693,
		  "critical_threshold": 1.9588528112516932,
		  "duration": 5,
		  "id": "2bj...",
		  "is_open": true,
		  "metric_label": "MetricName",
		  "metric_value": 2.255356387321597,
		  "monitor_name": "MonitorName",
		  "monitor_operator": "\u003e",
		  "opened_at": 1473129912,
		  "status": "critical",
		  "trigger": "monitor",
		  "url": "https://mackerel.io/orgs/.../alerts/2bj...",
		  "warning_threshold": 1.4665636369580741
		},
		"event": "alert",
		"host": {
		  "id": "22D4...",
		  "is_retired": false,
		  "memo": "",
		  "name": "app01",
		  "roles": [
			{
			  "fullname": "Service: Role",
			  "role_name": "Role",
			  "role_url": "https://mackerel.io/orgs/.../services/...",
			  "service_name": "Service",
			  "service_url": "https://mackerel.io/orgs/.../services/..."
			}
		  ],
		  "status": "working",
		  "type": "unknown",
		  "url": "https://mackerel.io/orgs/.../hosts/..."
		},
		"image_url": "https://mackerel.io/embed/public/.../....png",
		"memo": "memo....",
		"org_name": "Macker..."
	  }`
	require.JSONEq(t, expected, actual)
}

func TestExtructSection(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "rule.hoge",
			input: `
hogeareaerlkwarewk

## rule.hoge
Subsection content.

#### hoge

## AnotherApp
Another section content.
`,
			expected: `## rule.hoge
Subsection content.

#### hoge
`,
		},
		{
			name: "rule.fuga",
			input: `
dareawklfarhkjakjfa
コレは手打ちの文字

## rule.fuga

ここにはruleのmemo
後ろになにもないことも
`,
			expected: `## rule.fuga

ここにはruleのmemo
後ろになにもないことも
`,
		},
		{
			name: "rule.piyo",
			input: `
## rule.piyo
ここにはruleのmemo
後ろになにもないことも
`,
			expected: `## rule.piyo
ここにはruleのmemo
後ろになにもないことも
`,
		},
		{
			name:     "rule.hoge",
			input:    ``,
			expected: ``,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := prepalert.ExtructSection(c.input, "## "+c.name)
			require.Equal(t, c.expected, actual)
		})
	}
}
