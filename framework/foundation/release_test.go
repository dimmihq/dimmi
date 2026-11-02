//go:build !dev

package foundation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console"
)

func TestReleaseCommands(t *testing.T) {
	t.Parallel()

	t.Run("leaves the development commands out of release builds", func(t *testing.T) {
		t.Parallel()

		code, stdout, _ := run(t, options(nil), "")
		require.Equal(t, console.ExitSuccess, code)
		assert.NotContains(t, stdout, "make:")
		assert.NotContains(t, stdout, "route:lint")

		code, _, stderr := run(t, options(nil), "", "make:job", "SendReceipt")
		assert.Equal(t, console.ExitUsage, code)
		assert.Contains(t, stderr, "unknown command")
	})
}
