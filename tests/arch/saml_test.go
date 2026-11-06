package arch_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSAMLBinarySize(t *testing.T) {
	t.Parallel()

	t.Run("linking the saml library keeps the linker's dead method elimination", func(t *testing.T) {
		t.Parallel()

		cmd := exec.CommandContext(t.Context(), "go", "build", "-ldflags=-dumpdep", "-o", "/dev/null",
			"./testdata/samlapp")
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")

		dump, err := cmd.CombinedOutput()
		require.NoError(t, err, string(dump))
		require.Contains(t, string(dump), "saml.(*Provider).Consume", "the probe did not link the saml wrapper")

		assert.Empty(t, flagged(dump), "functions that disable dead method elimination")
	})
}
