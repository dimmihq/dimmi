package testkit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestTempFile(t *testing.T) {
	t.Parallel()

	t.Run("writes the content under the name in a directory of the test's own", func(t *testing.T) {
		t.Parallel()

		path := testkit.TempFile(t, "hot", "http://localhost:5173")

		assert.Equal(t, "hot", filepath.Base(path))
		content, err := os.ReadFile(filepath.Clean(path))
		require.NoError(t, err)
		assert.Equal(t, "http://localhost:5173", string(content))
	})
}

func TestGolden(t *testing.T) {
	t.Run("passes on the same output and writes it when asked", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out", "doc.json")
		t.Setenv("UPDATE_GOLDEN", "1")
		testkit.Golden(t, path, []byte("{}\n"))

		t.Setenv("UPDATE_GOLDEN", "")
		testkit.Golden(t, path, []byte("{}"))
	})

	t.Run("fails on a different output", func(t *testing.T) {
		path := testkit.TempFile(t, "doc.json", "{}")
		probe := &testing.T{}

		testkit.Golden(probe, path, []byte(`{"changed":true}`))

		assert.True(t, probe.Failed())
	})
}
