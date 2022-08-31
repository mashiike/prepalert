package prepalert_test

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func LoadFile(t *testing.T, path string) []byte {
	t.Helper()
	fp, err := os.Open(path)
	require.NoError(t, err)
	defer fp.Close()
	bs, err := io.ReadAll(fp)
	require.NoError(t, err)
	return bs
}

func LoadJSON[V any](t *testing.T, path string) V {
	t.Helper()
	var v V
	err := json.Unmarshal(LoadFile(t, path), &v)
	require.NoError(t, err)
	return v
}
