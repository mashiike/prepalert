package prepalert_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/prepalert"
	"github.com/stretchr/testify/require"
)

func LoadApp(t *testing.T, path string) *prepalert.App {
	t.Helper()
	app := prepalert.New("dummy-api-key")
	err := app.LoadConfig(path)
	require.NoError(t, err)
	t.Cleanup(func() {
		app.Close()
	})
	return app
}

func LoadFile(t *testing.T, path string) []byte {
	t.Helper()
	fp, err := os.Open(path)
	require.NoError(t, err)
	defer fp.Close()
	bs, err := io.ReadAll(fp)
	require.NoError(t, err)
	return bs
}

func LoadFileAsReader(t *testing.T, path string) io.Reader {
	t.Helper()
	bs := LoadFile(t, path)
	return bytes.NewReader(bs)
}

func LoadJSON[V any](t *testing.T, path string) V {
	t.Helper()
	var v V
	err := json.Unmarshal(LoadFile(t, path), &v)
	require.NoError(t, err)
	return v
}

func ParseExpression(t *testing.T, expr string) hcl.Expression {
	t.Helper()
	parsed, diags := hclsyntax.ParseExpression([]byte(expr), "expression.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		var buf strings.Builder
		w := hcl.NewDiagnosticTextWriter(&buf, map[string]*hcl.File{
			"expression.hcl": {
				Body:  nil,
				Bytes: []byte(expr),
			},
		}, 400, false)
		w.WriteDiagnostics(diags)
		t.Log(buf.String())
		require.FailNow(t, diags.Error())
	}
	return parsed
}
