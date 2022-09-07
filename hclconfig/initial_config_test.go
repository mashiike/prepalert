package hclconfig

import (
	"fmt"
	"os"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/require"
)

func TestGenerateInitialConfig(t *testing.T) {
	actual, err := GenerateInitialConfig("v0.2.0", "test-sqs", "test-service")
	require.NoError(t, err)
	expectedFilePath := "testdata/initial_config.hcl"
	expected, err := os.ReadFile(expectedFilePath)
	if err != nil {
		require.FailNow(t, fmt.Sprintf("cannot read expetec file [%s]: %v", expectedFilePath, err))
	}

	if string(expected) != string(actual) {
		dmp := diffmatchpatch.New()
		a, b, c := dmp.DiffLinesToChars(string(expected), string(actual))
		diffs := dmp.DiffMain(a, b, false)
		diffs = dmp.DiffCharsToLines(diffs, c)
		t.Logf("diff:\n%s", dmp.DiffPrettyText(diffs))
		require.FailNow(t, "missmatch generated config")
	}
}
