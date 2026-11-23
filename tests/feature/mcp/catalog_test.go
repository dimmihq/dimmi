package mcp_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/mcpserver"
	"github.com/dimmihq/dimmi/tests"
)

func TestCatalog(t *testing.T) {
	t.Parallel()

	tc := tests.New(t)
	report, err := mcpserver.Audit(tc.App.Router)
	require.NoError(t, err)

	t.Run("describes every API operation an agent can call", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Undocumented, "add catalog entries for:\n"+strings.Join(report.Undocumented, "\n"))
	})

	t.Run("keeps no entry for an operation that is gone", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Stale)
	})

	t.Run("gives every entry a title and a use", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Incomplete)
	})

	t.Run("names only tools that exist", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Unknown)
	})

	t.Run("gives every tool its own name and every argument a clear one", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Duplicate)
		assert.Empty(t, report.Unclear)
	})

	t.Run("leaves no argument with only what its name and type say", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, report.Unwritten, "write a text in app/mcpserver/argument_texts*.go for:\n"+
			strings.Join(report.Unwritten, "\n"))
	})

	t.Run("finds nothing wrong", func(t *testing.T) {
		t.Parallel()
		assert.True(t, report.Clean())
	})
}
