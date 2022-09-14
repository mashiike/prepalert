package queryrunner_test

import (
	"testing"

	"github.com/mashiike/prepalert/queryrunner"
	"github.com/stretchr/testify/require"
)

func TestNewQueryResultWithJSONLines(t *testing.T) {
	lines := [][]byte{
		[]byte(`{"name":"hoge"}`),
		[]byte(`{"name":"fuga", "age": 18}`),
		[]byte(`{"age": 82, "name":"piyo"}`),
		[]byte(`{"name":"tora", "memo": "animal"}`),
	}
	qr := queryrunner.NewQueryResultWithJSONLines("dummy", "SELECT * FROM dummy", lines)
	expected := &queryrunner.QueryResult{
		Name:    "dummy",
		Query:   "SELECT * FROM dummy",
		Columns: []string{"name", "age", "memo"},
		Rows: [][]string{
			{"hoge", "", ""},
			{"fuga", "18", ""},
			{"piyo", "82", ""},
			{"tora", "", "animal"},
		},
	}
	require.EqualValues(t, expected, qr)
}
