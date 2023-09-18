package s3select_test

import (
	context "context"
	"encoding/json"
	"fmt"
	io "io"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/mashiike/prepalert/provider"
	"github.com/mashiike/prepalert/provider/providertest"
	"github.com/mashiike/prepalert/provider/s3select"
	s3selectsqldriver "github.com/mashiike/s3-select-sql-driver"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=$GOFILE -destination=./mock_client_test.go -package=s3select_test
type S3SelectClient interface {
	s3selectsqldriver.S3SelectClient
}

func TestProvider(t *testing.T) {
	restore := flextime.Fix(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	defer restore()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	original := s3selectsqldriver.S3SelectClientConstructor
	t.Cleanup(func() {
		s3selectsqldriver.S3SelectClientConstructor = original
	})
	mockClient := NewMockS3SelectClient(ctrl)
	s3selectsqldriver.S3SelectClientConstructor = func(
		ctx context.Context,
		cfg *s3selectsqldriver.S3SelectConfig,
	) (s3selectsqldriver.S3SelectClient, error) {
		require.Equal(t, "test-bucket", cfg.BucketName)
		require.Equal(t, "test-key-prefix/2020/01/02/03/", cfg.ObjectKeyPrefix)
		return mockClient, nil
	}
	p, err := s3select.NewProvider(&provider.ProviderParameter{
		Type: "s3_select",
		Name: "default",
		Params: json.RawMessage(`{
			"region":"ap-northeast-1"
		}`),
	})
	require.NoError(t, err)

	hclBody := []byte(`
	expression = "select * from s3object s where s.timestamp > ?"
	params = [
		strftime_in_zone("%Y-%m-%dT%H:%M:%S.000Z","UTC", var.start_at),
	]
	bucket_name = "test-bucket"
	object_key_prefix = "test-key-prefix/${strftime_in_zone("%Y/%m/%d/%H/", "UTC", var.start_at)}"
	input_serialization {
		compression_type = "GZIP"
		json {
			type = "LINES"
		}
	}
	`)
	q, err := providertest.NewQuery(p, "test-query", hclBody, nil)
	require.NoError(t, err)

	mockClient.EXPECT().ListObjectsV2(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, input *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			require.Equal(t, "test-bucket", *input.Bucket)
			require.Equal(t, "test-key-prefix/2020/01/02/03/", *input.Prefix)
			return &s3.ListObjectsV2Output{
				Name: aws.String("test-bucket"),
				Contents: []types.Object{
					{
						Key:          aws.String("test-key-prefix/2020/01/02/03/00/00/00.gz"),
						LastModified: aws.Time(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
					},
					{
						Key:          aws.String("test-key-prefix/2020/01/02/03/00/00/01.gz"),
						LastModified: aws.Time(time.Date(2020, 1, 1, 0, 0, 1, 0, time.UTC)),
					},
				},
			}, nil
		},
	).Times(1)
	callCount := 0
	mockClient.EXPECT().SelectObjectContentWithWriter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, w io.Writer, params *s3.SelectObjectContentInput, optFns ...func(*s3.Options)) error {
			callCount++
			require.Equal(t, "test-bucket", *params.Bucket)
			require.EqualValues(t, &types.InputSerialization{
				CompressionType: types.CompressionTypeGzip,
				JSON: &types.JSONInput{
					Type: types.JSONTypeLines,
				},
			}, params.InputSerialization)
			require.EqualValues(t, &types.OutputSerialization{
				JSON: &types.JSONOutput{},
			}, params.OutputSerialization)
			switch callCount {
			case 1:
				require.Equal(t, "test-key-prefix/2020/01/02/03/00/00/00.gz", *params.Key)
				fmt.Fprintf(w, `{"id":1,"timestamp":"2020-01-01T00:00:00.000Z","message":"foo"}%s`, "\n")
				fmt.Fprintf(w, `{"id":2,"timestamp":"2020-01-01T00:00:01.000Z","message":"bar"}%s`, "\n")
			case 2:
				require.Equal(t, "test-key-prefix/2020/01/02/03/00/00/01.gz", *params.Key)
				fmt.Fprintf(w, `{"id":3,"timestamp":"2020-01-01T00:00:02.000Z","message":"baz"}%s`, "\n")
				fmt.Fprintf(w, `{"id":4,"timestamp":"2020-01-01T00:00:03.000Z","message":"qux"}%s`, "\n")
			default:
				require.Fail(t, "unexpected call count", callCount)
				return err
			}
			return nil
		},
	).Times(2)
	qr, err := providertest.RunQuery(context.Background(), q, map[string]interface{}{
		"var": map[string]interface{}{
			"start_at": time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC).Unix(),
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, &provider.QueryResult{
		Name:  "test-query",
		Query: "select * from s3object s where s.timestamp > ?",
		Params: []interface{}{
			"2020-01-02T03:04:05.000Z",
		},
		Columns: []string{"id", "timestamp", "message"},
		Rows: [][]json.RawMessage{
			{json.RawMessage(`1`), json.RawMessage(`"2020-01-01T00:00:00Z"`), json.RawMessage(`"foo"`)},
			{json.RawMessage(`2`), json.RawMessage(`"2020-01-01T00:00:01Z"`), json.RawMessage(`"bar"`)},
			{json.RawMessage(`3`), json.RawMessage(`"2020-01-01T00:00:02Z"`), json.RawMessage(`"baz"`)},
			{json.RawMessage(`4`), json.RawMessage(`"2020-01-01T00:00:03Z"`), json.RawMessage(`"qux"`)},
		},
	}, qr)
}
