package prepalert_test

import (
	"testing"

	"github.com/mashiike/prepalert"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

func TestGenerateInitialConfig(t *testing.T) {
	originalVersion := prepalert.Version
	prepalert.Version = "v0.12.0"
	t.Cleanup(func() {
		prepalert.Version = originalVersion
	})
	actual, err := prepalert.GenerateInitialConfig("test-sqs")
	require.NoError(t, err)
	g := goldie.New(t, goldie.WithFixtureDir("testdata/fixture"), goldie.WithNameSuffix(".golden.hcl"))
	g.Assert(t, "generate_initial_config", actual)
}
