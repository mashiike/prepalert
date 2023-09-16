package redshiftdata_test

import (
	context "context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsredshiftdata "github.com/aws/aws-sdk-go-v2/service/redshiftdata"
	"github.com/aws/aws-sdk-go-v2/service/redshiftdata/types"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/provider/redshiftdata"
	"github.com/mashiike/prepalert/provider/sqlprovider"
	redshiftdatasqldriver "github.com/mashiike/redshift-data-sql-driver"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=$GOFILE -destination=./mock_client_test.go -package=redshiftdata_test
type RedshiftDataClient interface {
	redshiftdatasqldriver.RedshiftDataClient
}

func TestProvider(t *testing.T) {
	restore := flextime.Fix(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	defer restore()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	original := redshiftdatasqldriver.RedshiftDataClientConstructor
	t.Cleanup(func() {
		redshiftdatasqldriver.RedshiftDataClientConstructor = original
	})
	mockClient := NewMockRedshiftDataClient(ctrl)
	redshiftdatasqldriver.RedshiftDataClientConstructor = func(ctx context.Context, cfg *redshiftdatasqldriver.RedshiftDataConfig) (redshiftdatasqldriver.RedshiftDataClient, error) {
		require.Equal(t, "wherehouse", *cfg.ClusterIdentifier)
		require.Equal(t, "test", *cfg.Database)
		require.Equal(t, "test", *cfg.DbUser)
		return mockClient, nil
	}
	mockClient.EXPECT().ExecuteStatement(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params *awsredshiftdata.ExecuteStatementInput, optFns ...func(*awsredshiftdata.Options)) (*awsredshiftdata.ExecuteStatementOutput, error) {
			require.Equal(t, "wherehouse", *params.ClusterIdentifier)
			require.Equal(t, "test", *params.Database)
			require.Equal(t, "test", *params.DbUser)
			require.Equal(t, "SELECT * FROM logs WHERE access_at = :start_at", *params.Sql)
			require.EqualValues(t, []types.SqlParameter{
				{
					Name:  aws.String("start_at"),
					Value: aws.String("2020-01-01"),
				},
			}, params.Parameters)
			return &awsredshiftdata.ExecuteStatementOutput{
				Id: aws.String("query-id"),
			}, nil
		},
	).Times(1)
	mockClient.EXPECT().DescribeStatement(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params *awsredshiftdata.DescribeStatementInput, optFns ...func(*awsredshiftdata.Options)) (*awsredshiftdata.DescribeStatementOutput, error) {
			require.Equal(t, "query-id", *params.Id)
			return &awsredshiftdata.DescribeStatementOutput{
				Id:           aws.String("query-id"),
				ResultRows:   3,
				Status:       types.StatusStringFinished,
				HasResultSet: aws.Bool(true),
			}, nil
		},
	).Times(1)
	mockClient.EXPECT().GetStatementResult(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, params *awsredshiftdata.GetStatementResultInput, optFns ...func(*awsredshiftdata.Options)) (*awsredshiftdata.GetStatementResultOutput, error) {
			require.Equal(t, "query-id", *params.Id)
			return &awsredshiftdata.GetStatementResultOutput{
				ColumnMetadata: []types.ColumnMetadata{
					{
						Name:     aws.String("id"),
						TypeName: aws.String("integer"),
					},
					{
						Name:     aws.String("name"),
						TypeName: aws.String("varchar"),
					},
				},
				Records: [][]types.Field{
					{
						&types.FieldMemberLongValue{
							Value: 1,
						},
						&types.FieldMemberStringValue{
							Value: "foo",
						},
					},
					{
						&types.FieldMemberLongValue{
							Value: 2,
						},
						&types.FieldMemberStringValue{
							Value: "bar",
						},
					},
					{
						&types.FieldMemberLongValue{
							Value: 3,
						},
						&types.FieldMemberStringValue{
							Value: "baz",
						},
					},
				},
			}, nil
		},
	).Times(1)
	p, err := redshiftdata.NewProvider(&prepalert.ProviderParameter{
		Type: "redshift_data",
		Name: "default",
		Params: json.RawMessage(`{
			"cluster_identifier":"wherehouse",
			"database":"test"
			,"db_user":"test"
			,"polling_interval":1
		}`),
	})
	require.NoError(t, err)
	defer p.Close()
	hclBody := []byte(`
sql = "SELECT * FROM ${var.table} WHERE access_at = :start_at"
params = {
	start_at = "2020-01-01"
}
`)
	q, err := sqlprovider.NewQuery(p, "test-query", hclBody, nil)
	require.NoError(t, err)
	qr, err := sqlprovider.RunQuery(context.Background(), q, map[string]interface{}{
		"var": map[string]interface{}{
			"table": "logs",
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, &prepalert.QueryResult{
		Name:  "test-query",
		Query: "SELECT * FROM logs WHERE access_at = :start_at",
		Params: []interface{}{
			sql.Named("start_at", "2020-01-01"),
		},
		Columns: []string{"id", "name"},
		Rows: [][]json.RawMessage{
			{json.RawMessage(`1`), json.RawMessage(`"foo"`)},
			{json.RawMessage(`2`), json.RawMessage(`"bar"`)},
			{json.RawMessage(`3`), json.RawMessage(`"baz"`)},
		},
	}, qr)
}
