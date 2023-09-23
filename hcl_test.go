package prepalert_test

import (
	"encoding/json"
	"testing"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/mock"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHCLGetMonitor(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mock.NewMockMackerelClient(ctrl)
	mockClient.EXPECT().GetAlert(gomock.Any()).Return(&mackerel.Alert{
		MonitorID: "mon-xxxxxxxxxxxxx",
	}, nil).Times(1)
	mockClient.EXPECT().GetMonitor("mon-xxxxxxxxxxxxx").Return(&mackerel.MonitorHostMetric{
		ID:   "hoge",
		Name: "fuga",
	}, nil).Times(1)
	app := LoadApp(t, "testdata/config/simple.hcl")
	app.SetMackerelClient(mockClient)
	webhook := app.MackerelService().NewExampleWebhookBody()
	evalCtx, err := app.NewEvalContext(webhook)
	require.NoError(t, err)
	expr, _ := hclutil.ParseExpression([]byte("jsonencode(get_monitor(webhook.alert))"))
	result, _ := expr.Value(evalCtx)
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden.json"))
	g.AssertJson(t, "hcl_get_monitor", json.RawMessage(result.AsString()))
}

type unmarshalAndMarshalTestCase string

func (c unmarshalAndMarshalTestCase) Run(t *testing.T) {
	t.Helper()
	body := LoadJSON[*prepalert.WebhookBody](t, string(c))
	ctyValue, err := hclutil.MarshalCTYValue(body)
	require.NoError(t, err)
	var actual prepalert.WebhookBody
	err = hclutil.UnmarshalCTYValue(ctyValue, &actual)
	require.NoError(t, err)

	acutalJSON, err := json.Marshal(actual)
	require.NoError(t, err)
	expectedJSON, err := json.Marshal(body)
	require.NoError(t, err)
	t.Log("actual", string(acutalJSON))
	t.Log("expected", string(expectedJSON))
	require.JSONEq(t, string(expectedJSON), string(acutalJSON))
}

func TestUnmarshalAndMarshalWebhookBody__Example(t *testing.T) {
	unmarshalAndMarshalTestCase("example_webhook.json").Run(t)
}

func TestUnmarshalAndMarshalWebhookBody__AcutalAlert(t *testing.T) {
	unmarshalAndMarshalTestCase("testdata/events_with_host.json").Run(t)
}
