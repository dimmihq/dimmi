package foundation_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
)

/*
TestMainEntryPoint is not parallel: it swaps the process arguments, which only
Main reads.
*/
func TestMainEntryPoint(t *testing.T) {
	t.Run("runs the command named by the process arguments", func(t *testing.T) {
		args := os.Args
		t.Cleanup(func() { os.Args = args })
		os.Args = []string{"app", "no-such-command"}

		code := foundation.Main(options(nil))

		assert.NotEqual(t, console.ExitSuccess, code)
	})
}
