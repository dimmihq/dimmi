/*
|--------------------------------------------------------------------------
| Disk Contract: Listing, Paths and Signed URLs
|--------------------------------------------------------------------------
|
| Listing must yield files in path order on every driver, paths that could
| escape a disk must be refused before storage is touched, and signed URLs
| must point at the requested file.
|
*/

package drivertest

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
)

func runListing(t *testing.T, newDisk NewDisk) {
	t.Helper()

	t.Run("lists files under a prefix in path order", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		for _, path := range []string{"docs/b.txt", "docs/a.txt", "docs/sub/c.txt", "other/d.txt"} {
			require.NoError(t, disk.Put(t.Context(), path, []byte(path), filesystem.PutOptions{}))
		}

		assert.Equal(t, []string{"docs/a.txt", "docs/b.txt", "docs/sub/c.txt"}, list(t, disk, "docs/"))
		assert.Len(t, list(t, disk, ""), 4)
		assert.Empty(t, list(t, disk, "none/"))
	})

	t.Run("reports sizes", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		require.NoError(t, disk.Put(t.Context(), "five", []byte("12345"), filesystem.PutOptions{}))

		for entry, err := range disk.List(t.Context(), "") {
			require.NoError(t, err)
			assert.Equal(t, filesystem.Entry{Path: "five", Size: 5, LastModified: entry.LastModified}, entry)
		}
	})

	t.Run("stops when the caller stops", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		for _, path := range []string{"1", "2", "3"} {
			require.NoError(t, disk.Put(t.Context(), path, []byte(path), filesystem.PutOptions{}))
		}

		seen := 0
		for _, err := range disk.List(t.Context(), "") {
			require.NoError(t, err)
			seen++

			break
		}

		assert.Equal(t, 1, seen)
	})
}

func runPaths(t *testing.T, newDisk NewDisk) {
	t.Helper()

	t.Run("rejects paths that could escape the disk", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		ctx := t.Context()

		for _, path := range []string{"", "/etc/passwd", "../up", "a/../../b", "a//b", "./a", "dir/", "."} {
			require.ErrorIs(t, disk.Put(ctx, path, nil, filesystem.PutOptions{}), filesystem.ErrInvalidPath, path)
			require.ErrorIs(t, disk.PutStream(ctx, path, strings.NewReader(""), filesystem.PutOptions{}),
				filesystem.ErrInvalidPath, path)
			_, err := disk.Get(ctx, path)
			require.ErrorIs(t, err, filesystem.ErrInvalidPath, path)
			_, err = disk.Open(ctx, path)
			require.ErrorIs(t, err, filesystem.ErrInvalidPath, path)
			_, err = disk.Exists(ctx, path)
			require.ErrorIs(t, err, filesystem.ErrInvalidPath, path)
			_, err = disk.Stat(ctx, path)
			require.ErrorIs(t, err, filesystem.ErrInvalidPath, path)
			require.ErrorIs(t, disk.Delete(ctx, path), filesystem.ErrInvalidPath, path)
			_, err = disk.URL(path)
			require.ErrorIs(t, err, filesystem.ErrInvalidPath, path)
			_, err = disk.TemporaryURL(ctx, path, time.Minute)
			require.ErrorIs(t, err, filesystem.ErrInvalidPath, path)
		}
	})

	t.Run("rejects a prefix that could escape the disk", func(t *testing.T) {
		t.Parallel()

		for _, err := range newDisk(t).List(t.Context(), "../") {
			require.ErrorIs(t, err, filesystem.ErrInvalidPath)
		}
	})
}

func runSignedURLs(t *testing.T, newDisk NewDisk) {
	t.Helper()

	t.Run("signs read and upload urls for a path", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		require.NoError(t, disk.Put(t.Context(), "reports/q1 final.pdf", []byte("%PDF"), filesystem.PutOptions{}))

		read, err := disk.TemporaryURL(t.Context(), "reports/q1 final.pdf", 5*time.Minute)
		require.NoError(t, err)
		upload, err := disk.TemporaryUploadURL(t.Context(), "reports/q2.pdf", filesystem.UploadURLOptions{
			TTL:         5 * time.Minute,
			ContentType: "application/pdf",
		})
		require.NoError(t, err)

		assertSignedURL(t, read, "q1")
		assertSignedURL(t, upload, "q2")
		assert.NotEqual(t, read, upload)
	})
}

func list(t *testing.T, disk filesystem.Filesystem, prefix string) []string {
	t.Helper()

	var paths []string
	for entry, err := range disk.List(t.Context(), prefix) {
		require.NoError(t, err)
		paths = append(paths, entry.Path)
	}

	return paths
}

func assertSignedURL(t *testing.T, raw, pathPart string) {
	t.Helper()

	u, err := url.Parse(raw)
	require.NoError(t, err)
	assert.NotEmpty(t, u.Host, raw)
	assert.Contains(t, u.String(), pathPart)
	assert.NotEmpty(t, u.RawQuery, "signature in query: %s", raw)
}
