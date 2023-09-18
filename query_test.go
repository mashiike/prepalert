package prepalert_test

import (
	"encoding/json"
	"testing"

	"github.com/mashiike/prepalert"
	"github.com/stretchr/testify/require"
)

func TestNewQueryResultWithJSONLines(t *testing.T) {
	lines := []map[string]json.RawMessage{
		{"name": json.RawMessage(`"hoge"`)},
		{"name": json.RawMessage(`"fuga"`), "age": json.RawMessage(`"18"`)},
		{"name": json.RawMessage(`"piyo"`), "age": json.RawMessage(`"82"`)},
		{"name": json.RawMessage(`"tora"`), "memo": json.RawMessage(`"animal"`)},
	}
	qr := prepalert.NewQueryResultWithJSONLines("dummy", "SELECT * FROM dummy", nil, lines...)
	expected := &prepalert.QueryResult{
		Name:    "dummy",
		Query:   "SELECT * FROM dummy",
		Columns: []string{"name", "age", "memo"},
		Rows: [][]json.RawMessage{
			{json.RawMessage(`"hoge"`), nil, nil},
			{json.RawMessage(`"fuga"`), json.RawMessage(`"18"`), nil},
			{json.RawMessage(`"piyo"`), json.RawMessage(`"82"`), nil},
			{json.RawMessage(`"tora"`), nil, json.RawMessage(`"animal"`)},
		},
	}
	require.EqualValues(t, expected, qr)
}
