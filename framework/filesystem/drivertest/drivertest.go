/*
|--------------------------------------------------------------------------
| Disk Contract
|--------------------------------------------------------------------------
|
| Every disk driver must behave the same, or moving from local storage to
| S3 would change what the application sees. A driver's tests call Run
| with a constructor for empty disks, and the same suite runs against
| every driver. The disks it gets must be able to sign temporary URLs.
|
|   func TestDisk(t *testing.T) {
|       drivertest.Run(t, newDisk)
|   }
|
*/

package drivertest

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/filesystem"
)

/*
NewDisk returns an empty disk that no other test uses.
*/
type NewDisk func(t *testing.T) filesystem.Filesystem

/*
Run runs the contract against disks from the constructor.
*/
func Run(t *testing.T, newDisk NewDisk) {
	t.Helper()

	t.Run("files", func(t *testing.T) { t.Parallel(); runFiles(t, newDisk) })
	t.Run("streams", func(t *testing.T) { t.Parallel(); runStreams(t, newDisk) })
	t.Run("attributes", func(t *testing.T) { t.Parallel(); runAttributes(t, newDisk) })
	t.Run("listing", func(t *testing.T) { t.Parallel(); runListing(t, newDisk) })
	t.Run("paths", func(t *testing.T) { t.Parallel(); runPaths(t, newDisk) })
	t.Run("signed urls", func(t *testing.T) { t.Parallel(); runSignedURLs(t, newDisk) })
}

func runFiles(t *testing.T, newDisk NewDisk) {
	t.Helper()

	t.Run("reads back what was put, creating folders", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)

		require.NoError(t, disk.Put(t.Context(), "a/b/c.txt", []byte("hello"), filesystem.PutOptions{}))

		got, err := disk.Get(t.Context(), "a/b/c.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello", string(got))

		exists, err := disk.Exists(t.Context(), "a/b/c.txt")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("reports a missing file", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)

		_, err := disk.Get(t.Context(), "missing.txt")
		require.ErrorIs(t, err, filesystem.ErrNotFound)
		_, err = disk.Open(t.Context(), "missing.txt")
		require.ErrorIs(t, err, filesystem.ErrNotFound)
		_, err = disk.Stat(t.Context(), "missing.txt")
		require.ErrorIs(t, err, filesystem.ErrNotFound)

		exists, err := disk.Exists(t.Context(), "missing.txt")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("deletes a file, missing or not", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		require.NoError(t, disk.Put(t.Context(), "gone.txt", []byte("x"), filesystem.PutOptions{}))

		require.NoError(t, disk.Delete(t.Context(), "gone.txt"))
		require.NoError(t, disk.Delete(t.Context(), "gone.txt"))

		_, err := disk.Get(t.Context(), "gone.txt")
		require.ErrorIs(t, err, filesystem.ErrNotFound)
	})
}

func runStreams(t *testing.T, newDisk NewDisk) {
	t.Helper()

	t.Run("streams a file in and out", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		body := strings.Repeat("x", 1<<20)

		require.NoError(t, disk.PutStream(t.Context(), "big.bin", strings.NewReader(body), filesystem.PutOptions{}))

		r, err := disk.Open(t.Context(), "big.bin")
		require.NoError(t, err)
		defer func() { require.NoError(t, r.Close()) }()
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, body, string(got))
	})

	t.Run("leaves no file when the stream fails", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		errRead := errors.New("read failed")

		err := disk.PutStream(t.Context(), "broken.bin", iotest.ErrReader(errRead), filesystem.PutOptions{})

		require.ErrorIs(t, err, errRead)
		exists, err := disk.Exists(t.Context(), "broken.bin")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("keeps the old file when a stream fails part way", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		errRead := errors.New("read failed")
		require.NoError(t, disk.Put(t.Context(), "kept.txt", []byte("old"), filesystem.PutOptions{
			ContentType: "text/plain",
		}))

		broken := io.MultiReader(strings.NewReader(strings.Repeat("new", 1000)), iotest.ErrReader(errRead))
		err := disk.PutStream(t.Context(), "kept.txt", broken, filesystem.PutOptions{ContentType: "text/csv"})

		require.ErrorIs(t, err, errRead)
		got, err := disk.Get(t.Context(), "kept.txt")
		require.NoError(t, err)
		assert.Equal(t, "old", string(got))
		attrs, err := disk.Stat(t.Context(), "kept.txt")
		require.NoError(t, err)
		assert.Equal(t, "text/plain", attrs.ContentType)
	})
}

func runAttributes(t *testing.T, newDisk NewDisk) {
	t.Helper()

	t.Run("keeps what was written with the file", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		before := clock.System().Now().Add(-time.Minute)

		require.NoError(t, disk.Put(t.Context(), "doc.json", []byte(`{"a":1}`), filesystem.PutOptions{
			ContentType:  "application/json",
			CacheControl: "max-age=60",
			Metadata:     map[string]string{"Owner": "u1"},
		}))

		attrs, err := disk.Stat(t.Context(), "doc.json")
		require.NoError(t, err)
		assert.Equal(t, int64(7), attrs.Size)
		assert.Equal(t, "application/json", attrs.ContentType)
		assert.Equal(t, "max-age=60", attrs.CacheControl)
		assert.Equal(t, map[string]string{"owner": "u1"}, attrs.Metadata)
		assert.True(t, attrs.LastModified.After(before), "last modified %v", attrs.LastModified)
	})

	t.Run("detects the content type when none is given", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

		require.NoError(t, disk.Put(t.Context(), "image", png, filesystem.PutOptions{}))

		attrs, err := disk.Stat(t.Context(), "image")
		require.NoError(t, err)
		assert.Equal(t, "image/png", attrs.ContentType)
	})
}
