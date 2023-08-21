package prepalert_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/mock"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"go.uber.org/mock/gomock"
)

func ptr[V any](v V) *V {
	return &v
}

func TestMackerelService__UpdateAlertMemo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().UpdateAlert("xxxxxxxxxxxxx", mackerel.UpdateAlertParam{
		Memo: "hoge",
	}).Return(
		&mackerel.UpdateAlertResponse{},
		nil,
	).Times(1)

	svc := prepalert.NewMackerelService(client)
	err := svc.UpdateAlertMemo(context.Background(), "xxxxxxxxxxxxx", "hoge")
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
	expected := `{
		"orgName": "test-org",
		"event": "alert",
		"alert": {
		  "id": "xxxxxxxxxxx",
		  "url": "https://mackerel.io/orgs/test-org/alerts/xxxxxxxxxxx",
		  "openedAt": 1691323531,
		  "closedAt": 1691323831,
		  "createdAt": 1691323531000,
		  "status": "ok",
		  "isOpen": false,
		  "trigger": "monitor",
		  "monitorName": "test-monitor",
		  "metricLabel": "custom.host.metric",
		  "metricValue": 0.5,
		  "warningThreshold": 5,
		  "monitorOperator": ">",
		  "duration": 3
		},
		"memo": "",
		"host": {
		  "name": "test-instance",
		  "memo": "",
		  "isRetired": false,
		  "id": "zzzzzzzzzzz",
		  "url": "https://mackerel.io/orgs/test-org/hosts/zzzzzzzzzzz",
		  "status": "working",
		  "roles": [
			{
			  "fullname": "prod: Instance",
			  "serviceName": "prod",
			  "roleName": "Instance",
			  "serviceUrl": "https://mackerel.io/orgs/test-org/services/prod",
			  "roleUrl": "https://mackerel.io/orgs/test-org/services/prod#role=Instance"
			}
		  ]
		},
		"imageUrl": null
	  }`
	bs, err := json.Marshal(actual)
	require.NoError(t, err)
	require.JSONEq(t, expected, string(bs))
}

func TestWebnookBody__MarshalCTYValues(t *testing.T) {
	body := LoadJSON[prepalert.WebhookBody](t, "testdata/event.json")
	expr, diags := hclsyntax.ParseExpression([]byte("jsonencode(test_event)"), "test.hcl", hcl.InitialPos)
	require.False(t, diags.HasErrors())
	ctx := hclconfig.NewEvalContext()
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
