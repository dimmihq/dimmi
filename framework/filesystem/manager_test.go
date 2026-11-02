package filesystem_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/manager"
)

func TestManager(t *testing.T) {
	t.Parallel()

	newManager := func(t *testing.T) *filesystem.Manager {
		t.Helper()

		m := filesystem.NewManager(filesystem.Config{
			Default: "attachments",
			Disks: map[string]config.Section{
				"attachments": {"driver": "local", "root": t.TempDir()},
				"exports":     {"driver": "local", "root": t.TempDir()},
			},
		})
		m.Extend("local", local.Factory)

		return m
	}

	t.Run("resolves named and default disks", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)

		attachments, err := m.Disk("attachments")
		require.NoError(t, err)
		def, err := m.Default()
		require.NoError(t, err)
		exports, err := m.Disk("exports")
		require.NoError(t, err)

		assert.Same(t, attachments, def)
		assert.NotSame(t, attachments, exports)
	})

	t.Run("keeps each disk's files apart", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)
		attachments, err := m.Disk("attachments")
		require.NoError(t, err)
		exports, err := m.Disk("exports")
		require.NoError(t, err)

		require.NoError(t, attachments.Put(t.Context(), "a.txt", []byte("x"), filesystem.PutOptions{}))

		exists, err := exports.Exists(t.Context(), "a.txt")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("reports an unknown disk", func(t *testing.T) {
		t.Parallel()

		_, err := newManager(t).Disk("nope")

		require.ErrorIs(t, err, manager.ErrUnknownInstance)
	})
}
