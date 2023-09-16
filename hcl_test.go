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
	app, err := prepalert.New("dummy-api-key")
	require.NoError(t, err)
	app.SetMackerelClient(mockClient)
	app.LoadConfig("testdata/config/simple.hcl")
	webhook := app.MackerelService().NewExampleWebhookBody()
	evalCtx, err := app.NewEvalContext(webhook)
	require.NoError(t, err)
	expr, _ := hclutil.ParseExpression([]byte("jsonencode(get_monitor(webhook.alert))"))
	result, _ := expr.Value(evalCtx)
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden.json"))
	g.AssertJson(t, "hcl_get_monitor", json.RawMessage(result.AsString()))
}
