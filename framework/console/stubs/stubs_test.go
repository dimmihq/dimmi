package stubs_test

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console/stubs"
)

func TestFS(t *testing.T) {
	t.Parallel()

	t.Run("holds a stub for every kind of file at its root", func(t *testing.T) {
		t.Parallel()

		names, err := fs.Glob(stubs.FS(), "*.tmpl")

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{
			"action.go.tmpl", "command.go.tmpl", "doc.go.tmpl", "event.go.tmpl", "job.go.tmpl", "listener.go.tmpl",
			"listener.queued.go.tmpl", "mail.go.tmpl", "migration.create.sql.tmpl", "migration.sql.tmpl",
			"test.go.tmpl", "controller.go.tmpl", "request.go.tmpl", "policy.go.tmpl", "notification.go.tmpl",
		}, names)
	})
}
