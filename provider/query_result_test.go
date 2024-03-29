package provider_test

import (
	"encoding/json"
	"testing"

	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert/provider"
	"github.com/stretchr/testify/require"
)

func TestNewQueryResultWithJSONLines(t *testing.T) {
	lines := []map[string]json.RawMessage{
		{"name": json.RawMessage(`"hoge"`)},
		{"name": json.RawMessage(`"fuga"`), "age": json.RawMessage(`"18"`)},
		{"name": json.RawMessage(`"piyo"`), "age": json.RawMessage(`"82"`)},
		{"name": json.RawMessage(`"tora"`), "memo": json.RawMessage(`"animal"`)},
	}
	qr := provider.NewQueryResultWithJSONLines("dummy", "SELECT * FROM dummy", nil, lines...)
	expected := &provider.QueryResult{
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

func TestMarshlCTYValue(t *testing.T) {
	lines := []map[string]json.RawMessage{
		{"name": json.RawMessage(`"hoge"`)},
		{"name": json.RawMessage(`"fuga"`), "age": json.RawMessage(`"18"`)},
		{"name": json.RawMessage(`"piyo"`), "age": json.RawMessage(`"82"`)},
		{"name": json.RawMessage(`"tora"`), "memo": json.RawMessage(`"animal"`)},
	}
	qr := provider.NewQueryResultWithJSONLines("dummy", "SELECT * FROM dummy", nil, lines...)
	v, err := hclutil.MarshalCTYValue(qr)
	require.NoError(t, err)
	val, err := hclutil.DumpCTYValue(v)
	require.NoError(t, err)
	t.Log(string(val))
	require.JSONEq(t, `{
		"name":"dummy",
		"query":"SELECT * FROM dummy",
		"params":[],
		"columns":["name","age","memo"],
		"rows":[
			["hoge",null,null],
			["fuga","18",null],
			["piyo","82",null],
			["tora",null,"animal"]
		]
	}`, val)
}

func TestEvalContextQueryVariablesMarshlCTYValue(t *testing.T) {
	t.Run("runnning", func(t *testing.T) {
		q := provider.EvalContextQueryVariables{
			FQN:    "query.hoge,fuga",
			Status: "running",
		}
		v, err := hclutil.MarshalCTYValue(q)
		require.NoError(t, err)
		val, err := hclutil.DumpCTYValue(v)
		require.NoError(t, err)
		t.Log(string(val))
		require.JSONEq(t, `{
		"fqn":"query.hoge,fuga",
		"status":"running",
		"error":"",
		"result":null
	}`, val)
	})
	t.Run("failed", func(t *testing.T) {
		q := provider.EvalContextQueryVariables{
			FQN:    "query.hoge,fuga",
			Status: "failed",
			Error:  "error",
		}
		v, err := hclutil.MarshalCTYValue(q)
		require.NoError(t, err)
		val, err := hclutil.DumpCTYValue(v)
		require.NoError(t, err)
		t.Log(string(val))
		require.JSONEq(t, `{
		"fqn":"query.hoge,fuga",
		"status":"failed",
		"error":"error",
		"result":null
	}`, val)
	})
	t.Run("success", func(t *testing.T) {
		q := provider.EvalContextQueryVariables{
			FQN:    "query.hoge,fuga",
			Status: "success",
			Result: provider.NewQueryResultWithJSONLines("dummy", "SELECT * FROM dummy", nil, map[string]json.RawMessage{
				"name": json.RawMessage(`"hoge"`),
			}),
		}
		v, err := hclutil.MarshalCTYValue(q)
		require.NoError(t, err)
		val, err := hclutil.DumpCTYValue(v)
		require.NoError(t, err)
		t.Log(string(val))
		require.JSONEq(t, `{
		"fqn":"query.hoge,fuga",
		"status":"success",
		"error":"",
		"result":{
			"name":"dummy",
			"query":"SELECT * FROM dummy",
			"params":[],
			"columns":["name"],
			"rows":[
				["hoge"]
			]
		}
	}`, val)
	})
}
