package prepalert_test

import (
	"context"
	"testing"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/mock"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestUpdater__NewMemo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("2bj...").Return(
		&mackerel.Alert{
			ID:        "2bj...",
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
			require.Equal(t, "2bj...", id)
			g.Assert(t, "updater_new_memo", []byte(param.Memo))
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)
	backend := mock.NewMockBackend(ctrl)
	backend.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("https://example.com/alerts/hoge.txt", true, nil).Times(1)

	svc := prepalert.NewMackerelService(client)
	body := LoadJSON[prepalert.WebhookBody](t, "example_webhook.json")
	u := svc.NewMackerelUpdater(&body, backend)
	u.AddMemoSectionText("rule.hoge", "hogehoge", nil)
	err := u.Flush(context.Background(), hclutil.NewEvalContext())
	require.NoError(t, err)
}

func TestUpdater__OtherAppSection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("2bj...").Return(
		&mackerel.Alert{
			ID:        "2bj...",
			Memo:      "header message written by human\n\n ## Other App Block\n\n ### hogehoge\n\n this message written by other app",
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
			require.Equal(t, "2bj...", id)
			g.Assert(t, "updater_other_app_section", []byte(param.Memo))
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)
	backend := mock.NewMockBackend(ctrl)
	backend.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("https://example.com/alerts/hoge.txt", true, nil).Times(1)

	svc := prepalert.NewMackerelService(client)
	body := LoadJSON[prepalert.WebhookBody](t, "example_webhook.json")
	u := svc.NewMackerelUpdater(&body, backend)
	u.AddMemoSectionText("rule.hoge", "hogehoge", nil)
	err := u.Flush(context.Background(), hclutil.NewEvalContext())
	require.NoError(t, err)
}

func TestUpdater__RewritePrepalertSection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture/"), goldie.WithNameSuffix(".golden"))
	client := mock.NewMockMackerelClient(ctrl)
	client.EXPECT().GetAlert("2bj...").Return(
		&mackerel.Alert{
			ID:        "2bj...",
			Memo:      string(LoadFile(t, "testdata/example_memo.md")),
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
			require.Equal(t, "2bj...", id)
			g.Assert(t, "updater_rewrite_prepalert_section", []byte(param.Memo))
			return mackerel.UpdateAlertResponse{}, nil
		},
	).Times(1)
	backend := mock.NewMockBackend(ctrl)
	backend.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("https://example.com/alerts/hoge.txt", true, nil).Times(1)

	svc := prepalert.NewMackerelService(client)
	body := LoadJSON[prepalert.WebhookBody](t, "example_webhook.json")
	u := svc.NewMackerelUpdater(&body, backend)
	u.AddMemoSectionText("rule.hoge", "hogehoge", nil)
	err := u.Flush(context.Background(), hclutil.NewEvalContext())
	require.NoError(t, err)
}
