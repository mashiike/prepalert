package queryrunner_test

import (
	"context"
	"log"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/mashiike/prepalert/queryrunner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyQueryRunner struct {
	name    string
	Columns []string `hcl:"columns"`
}

func (r *dummyQueryRunner) Name() string {
	return r.name
}

func (r *dummyQueryRunner) Type() string {
	return "dummy"
}

func (r *dummyQueryRunner) Prepare(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.PreparedQuery, hcl.Diagnostics) {
	log.Printf("[debug] prepare `%s` with s3_select query_runner", name)
	q := &dummyPreparedQuery{
		name:    name,
		columns: r.Columns,
	}
	diags := gohcl.DecodeBody(body, ctx, q)
	return q, diags

}

type dummyPreparedQuery struct {
	name    string
	columns []string

	Rows [][]string `hcl:"rows"`
}

func (q *dummyPreparedQuery) Name() string {
	return q.name
}

func (q *dummyPreparedQuery) Run(ctx context.Context, data interface{}) (*queryrunner.QueryResult, error) {
	return queryrunner.NewQueryResult(q.name, "", q.columns, q.Rows), nil
}

func TestDecodeBody(t *testing.T) {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName: "dummy",
		BuildQueryRunnerFunc: func(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
			runner := &dummyQueryRunner{
				name: name,
			}
			diags := gohcl.DecodeBody(body, ctx, runner)
			return runner, diags
		},
	})
	require.NoError(t, err)

	parser := hclparse.NewParser()
	src := []byte(`
	query_runner "dummy" "default" {
		columns = ["id", "name", "age"]
	}

	query "default" {
		runner = query_runner.dummy.default
		rows = [
			[ "1", "hoge", "13"],
			[ "2", "fuga", "26"],
		]
	}

	extra = "hoge"
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	queries, remain, diags := queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	if !assert.False(t, diags.HasErrors()) {
		var builder strings.Builder
		w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
		w.WriteDiagnostics(diags)
		t.Log(builder.String())
		t.FailNow()
	}
	attrs, _ := remain.JustAttributes()
	require.Equal(t, 1, len(attrs))
	query, ok := queries.Get("default")
	require.True(t, ok)
	result, err := query.Run(context.Background(), nil)
	require.NoError(t, err)
	expected := strings.TrimSpace(`
+----+------+-----+
| ID | NAME | AGE |
+----+------+-----+
|  1 | hoge |  13 |
|  2 | fuga |  26 |
+----+------+-----+
`)
	require.Equal(t, expected, strings.TrimSpace(result.ToTable()))
}

func TestDecodeBodyRequireQueryRunner(t *testing.T) {
	parser := hclparse.NewParser()
	src := []byte(`
	query "default" {}
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	_, _, diags = queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	require.True(t, diags.HasErrors(), "has errors")

	var builder strings.Builder
	w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
	w.WriteDiagnostics(diags)
	expected := `
Error: Missing required argument

  on config.hcl line 2, in query "default":
   2: 	query "default" {}

The argument "runner" is required, but no definition was found.`
	require.EqualValues(t, strings.TrimSpace(expected), strings.TrimSpace(builder.String()))
}

func TestDecodeBodyQueryRunnerNotFound(t *testing.T) {
	parser := hclparse.NewParser()
	src := []byte(`
	query "default" {
		runner = query_runner.not_found.default
	}
	`)
	file, diags := parser.ParseHCL(src, "config.hcl")
	require.False(t, diags.HasErrors())
	_, _, diags = queryrunner.DecodeBody(file.Body, &hcl.EvalContext{})
	require.True(t, diags.HasErrors(), "has errors")

	var builder strings.Builder
	w := hcl.NewDiagnosticTextWriter(&builder, parser.Files(), 400, false)
	w.WriteDiagnostics(diags)
	expected := `
Error: Invalid Relation

  on config.hcl line 3, in query "default":
   3: 		runner = query_runner.not_found.default

query_runner "not_found.default" is not found`
	require.EqualValues(t, strings.TrimSpace(expected), strings.TrimSpace(builder.String()))
}
