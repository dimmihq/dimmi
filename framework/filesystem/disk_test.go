package filesystem_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/filesystem/drivertest"
)

func newDisk(t *testing.T) filesystem.Filesystem {
	t.Helper()

	disk, err := local.New(local.Options{
		Root:          t.TempDir(),
		URL:           "https://cdn.example.com/",
		SignedURLBase: "https://app.example.com/files",
		SigningKey:    "test-signing-key",
	})
	require.NoError(t, err)

	return disk
}

func TestDisk(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newDisk)

	t.Run("escapes each segment of public urls", func(t *testing.T) {
		t.Parallel()

		link, err := newDisk(t).URL("a b/c?d.png")

		require.NoError(t, err)
		assert.Equal(t, "https://cdn.example.com/a%20b/c%3Fd.png", link)
		assert.Equal(t, "a%20b/c%3Fd.png", filesystem.EscapePath("a b/c?d.png"))
	})

	t.Run("lowercases metadata keys and drops empty metadata", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		upper := filesystem.PutOptions{Metadata: map[string]string{"X-ID": "1"}}
		require.NoError(t, disk.Put(t.Context(), "a", nil, upper))
		require.NoError(t, disk.Put(t.Context(), "b", nil, filesystem.PutOptions{Metadata: map[string]string{}}))

		a, err := disk.Stat(t.Context(), "a")
		require.NoError(t, err)
		b, err := disk.Stat(t.Context(), "b")
		require.NoError(t, err)

		assert.Equal(t, map[string]string{"x-id": "1"}, a.Metadata)
		assert.Nil(t, b.Metadata)
	})

	t.Run("reports store failures with the operation and path", func(t *testing.T) {
		t.Parallel()
		disk, err := local.New(local.Options{Root: t.TempDir()})
		require.NoError(t, err)
		require.NoError(t, disk.Close())

		_, err = disk.Exists(t.Context(), "a")
		require.ErrorContains(t, err, `check "a"`)
		_, err = disk.Get(t.Context(), "a")
		require.ErrorContains(t, err, `open "a"`)
		require.ErrorContains(t, disk.Delete(t.Context(), "a"), `delete "a"`)
		err = disk.PutStream(t.Context(), "a", strings.NewReader(""), filesystem.PutOptions{ContentType: "x/y"})
		require.ErrorContains(t, err, `put "a"`)
	})
}
