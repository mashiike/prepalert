package sqlprovider_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Songmu/flextime"
	"github.com/mashiike/prepalert"
	"github.com/mashiike/prepalert/provider/sqlprovider"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=$GOFILE -destination=./mock_$GOFILE -package=sqlprovider_test

type Driver struct {
	Conn *MockConn
}

var mockedSQLDriver = &Driver{}

func init() {
	sql.Register("__prepalert_test_mocked_db", mockedSQLDriver)
}

func (d *Driver) Open(name string) (driver.Conn, error) {
	if d.Conn != nil {
		return d.Conn, nil
	}
	return nil, errors.New("not set conn")
}

type Conn interface {
	driver.Conn
	driver.QueryerContext
}

type Rows interface {
	driver.Rows
}

func TestProvider(t *testing.T) {
	restore := flextime.Fix(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	defer restore()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockedSQLDriver.Conn = NewMockConn(ctrl)
	mockedRows := NewMockRows(ctrl)
	mockedSQLDriver.Conn.EXPECT().QueryContext(
		gomock.Any(),
		"SELECT * FROM logs WHERE access_at = ?",
		[]driver.NamedValue{
			{Ordinal: 1, Value: "2020-01-01"},
		},
	).Return(mockedRows, nil).Times(1)
	mockedSQLDriver.Conn.EXPECT().Close().Return(nil).Times(1)
	mockedRows.EXPECT().Columns().Return([]string{"id", "access_at", "user_id", "path"}).AnyTimes()
	calledCount := 0
	mockedRows.EXPECT().Next(gomock.Any()).DoAndReturn(
		func(dest []driver.Value) error {
			calledCount++
			switch calledCount {
			case 1:
				dest[0] = int64(1)
				dest[1] = "2020-01-01"
				dest[2] = "user1"
				dest[3] = "/path/to"
				return nil
			case 2:
				dest[0] = int64(2)
				dest[1] = "2020-01-01"
				dest[2] = "user2"
				dest[3] = "/path/to"
				return nil
			case 3:
				dest[0] = int64(3)
				dest[1] = "2020-01-01"
				dest[2] = "user3"
				dest[3] = "/path/to"
				return nil
			default:
				return io.EOF
			}
		},
	).AnyTimes()
	mockedRows.EXPECT().Close().Return(nil).Times(1)
	p, err := sqlprovider.NewProvider("__prepalert_test_mocked_db", "")
	require.NoError(t, err)
	defer p.Close()
	hclBody := []byte(`
sql = "SELECT * FROM ${var.table} WHERE access_at = ?"
params = [strftime("%Y-%m-%d", now())]
`)
	q, err := sqlprovider.NewQuery(p, "test-query", hclBody, nil)
	require.NoError(t, err)
	result, err := sqlprovider.RunQuery(context.Background(), q, map[string]interface{}{
		"var": map[string]interface{}{
			"table": "logs",
		},
	})
	require.NoError(t, err)
	require.EqualValues(t, &prepalert.QueryResult{
		Name:    "test-query",
		Query:   "SELECT * FROM logs WHERE access_at = ?",
		Params:  []interface{}{"2020-01-01"},
		Columns: []string{"id", "access_at", "user_id", "path"},
		Rows: [][]json.RawMessage{
			{json.RawMessage(`1`), json.RawMessage(`"2020-01-01"`), json.RawMessage(`"user1"`), json.RawMessage(`"/path/to"`)},
			{json.RawMessage(`2`), json.RawMessage(`"2020-01-01"`), json.RawMessage(`"user2"`), json.RawMessage(`"/path/to"`)},
			{json.RawMessage(`3`), json.RawMessage(`"2020-01-01"`), json.RawMessage(`"user3"`), json.RawMessage(`"/path/to"`)},
		},
	}, result)
}
