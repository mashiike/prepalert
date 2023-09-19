package cloudwatchlogsinsights_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cloudwatchlogsinsightsdriver "github.com/mashiike/cloudwatch-logs-insights-driver"
	"github.com/mashiike/prepalert/provider"
	"github.com/mashiike/prepalert/provider/cloudwatchlogsinsights"
	"github.com/mashiike/prepalert/provider/providertest"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=$GOFILE -destination=./mock_client_test.go -package=cloudwatchlogsinsights_test
type CloudwatchLogsClient interface {
	cloudwatchlogsinsightsdriver.CloudwatchLogsClient
}

func TestProvider(t *testing.T) {
	restore := flextime.Fix(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	defer restore()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := cloudwatchlogsinsightsdriver.CloudwatchLogsClientConstructor
	t.Cleanup(func() {
		cloudwatchlogsinsightsdriver.CloudwatchLogsClientConstructor = original
	})
	mockClient := NewMockCloudwatchLogsClient(ctrl)
	cloudwatchlogsinsightsdriver.CloudwatchLogsClientConstructor = func(
		ctx context.Context,
		cfg *cloudwatchlogsinsightsdriver.CloudwatchLogsInsightsConfig,
	) (cloudwatchlogsinsightsdriver.CloudwatchLogsClient, error) {
		require.Equal(t, "ap-northeast-1", cfg.Region)
		require.ElementsMatch(t, []string{"test-log-group"}, cfg.LogGroupNames)
		return mockClient, nil
	}
	p, err := cloudwatchlogsinsights.NewProvider(&provider.ProviderParameter{
		Type: "cloudwach_logs_insights",
		Name: "default",
		Params: json.RawMessage(`{
			"region":"ap-northeast-1",
			"default_log_group_names":["test-log-group"]
		}`),
	})
	require.NoError(t, err)
	defer p.Close()
	hclBody := []byte(`
query = "fields @timestamp, @message | limit 10"
start_time = var.start_at
end_time = var.end_at
log_group_names = [
	"test-log-group2",
]
`)
	q, err := providertest.NewQuery(p, "test-query", hclBody, nil)
	require.NoError(t, err)

	mockClient.EXPECT().StartQuery(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params *cloudwatchlogs.StartQueryInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.StartQueryOutput, error) {
			require.Equal(t, "fields @timestamp, @message | limit 10", *params.QueryString)
			require.Equal(t, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), *params.StartTime)
			require.Equal(t, time.Date(2020, 1, 1, 0, 5, 0, 0, time.UTC).Unix(), *params.EndTime)
			require.EqualValues(t, "test-log-group2", *params.LogGroupName)
			return &cloudwatchlogs.StartQueryOutput{
				QueryId: aws.String("query-id"),
			}, nil
		},
	).Times(1)
	mockClient.EXPECT().GetQueryResults(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params *cloudwatchlogs.GetQueryResultsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetQueryResultsOutput, error) {
			require.Equal(t, "query-id", *params.QueryId)
			return &cloudwatchlogs.GetQueryResultsOutput{
				Results: [][]types.ResultField{
					{
						{
							Field: aws.String("@timestamp"),
							Value: aws.String("2020-01-01T00:00:00.000Z"),
						},
						{
							Field: aws.String("@message"),
							Value: aws.String("foo"),
						},
					},
					{
						{
							Field: aws.String("@timestamp"),
							Value: aws.String("2020-01-01T00:00:01.000Z"),
						},
						{
							Field: aws.String("@message"),
							Value: aws.String("bar"),
						},
					},
				},
				Status: types.QueryStatusComplete,
			}, nil
		},
	).Times(1)

	qr, err := providertest.RunQuery(context.Background(), q, map[string]interface{}{
		"var": map[string]interface{}{
			"start_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
			"end_at":   time.Date(2020, 1, 1, 0, 5, 0, 0, time.UTC).Unix(),
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, &provider.QueryResult{
		Name:  "test-query",
		Query: "fields @timestamp, @message | limit 10",
		Params: []interface{}{
			sql.Named("start_time", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
			sql.Named("end_time", time.Date(2020, 1, 1, 0, 5, 0, 0, time.UTC)),
			sql.Named("log_group_name", "test-log-group2"),
		},
		Columns: []string{"@timestamp", "@message"},
		Rows: [][]json.RawMessage{
			{json.RawMessage(`"2020-01-01T00:00:00.000Z"`), json.RawMessage(`"foo"`)},
			{json.RawMessage(`"2020-01-01T00:00:01.000Z"`), json.RawMessage(`"bar"`)},
		},
	}, qr)
}
