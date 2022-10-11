package hclconfig

import (
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

func TestGenerateInitialConfig(t *testing.T) {
	actual, err := GenerateInitialConfig("v0.2.0", "test-sqs", "test-service")
	require.NoError(t, err)
	g := goldie.New(t, goldie.WithNameSuffix(".hcl"))
	g.Assert(t, "initial_config", actual)
}
