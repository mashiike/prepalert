package prepalert_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/canyon"
	"github.com/mashiike/canyon/canyontest"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/mashiike/prepalert/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestServeHTTPAsWebhookServer(t *testing.T) {
	cfg, err := hclconfig.Load("testdata/config/simple/", "current")
	require.NoError(t, err)
	app, err := prepalert.NewWithMackerelClient(nil, cfg)
	require.NoError(t, err)

	var sendMessageCount int
	var sendMessageRequestId string
	h := canyontest.AsServer(
		app,
		canyon.SQSMessageSenderFunc(func(r *http.Request, _ canyon.MessageAttributes) (string, error) {
			sendMessageCount++
			sendMessageRequestId = r.Header.Get(prepalert.HeaderRequestID)
			return "dummy-message-id", nil
		}),
	)
	r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "testdata/event.json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, sendMessageCount)
	require.Equal(t, resp.Header.Get(prepalert.HeaderRequestID), sendMessageRequestId)
}

func TestServeHTTPAsWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock.NewMockMackerelClient(ctrl)
	expectedMemo := "How do you respond to alerts?\nDescribe information about your alert response here.\n"
	client.EXPECT().UpdateAlert(gomock.Any(), gomock.Any()).DoAndReturn(
		func(alertID string, param mackerel.UpdateAlertParam) (*mackerel.UpdateAlertResponse, error) {
			require.Equal(t, "2bj...", alertID)
			require.Equal(t, expectedMemo, param.Memo)
			return &mackerel.UpdateAlertResponse{
				Memo: expectedMemo,
			}, nil
		},
	).Times(1)

	cfg, err := hclconfig.Load("testdata/config/simple/", "current")
	require.NoError(t, err)
	app, err := prepalert.NewWithMackerelClient(client, cfg)
	require.NoError(t, err)

	h := canyontest.AsWorker(app)
	r := httptest.NewRequest(http.MethodPost, "/", LoadFileAsReader(t, "testdata/event.json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
