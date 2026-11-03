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
