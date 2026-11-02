package local_test

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/local"
	"github.com/dimmihq/dimmi/framework/filesystem/drivertest"
)

func newDisk(t *testing.T) filesystem.Filesystem {
	t.Helper()

	disk, err := local.New(local.Options{
		Root:          t.TempDir(),
		SignedURLBase: "https://files.example.com/signed",
		SigningKey:    "test-signing-key",
	})
	require.NoError(t, err)

	return disk
}

func TestDisk(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newDisk)
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("stores files as plain files under the root", func(t *testing.T) {
		t.Parallel()
		root := filepath.Join(t.TempDir(), "created")
		disk, err := local.Factory("public", config.Section{"driver": "local", "root": root})
		require.NoError(t, err)

		require.NoError(t, disk.Put(t.Context(), "a/b.txt", []byte("hi"), filesystem.PutOptions{}))

		onDisk, err := fs.ReadFile(os.DirFS(root), "a/b.txt")
		require.NoError(t, err)
		assert.Equal(t, "hi", string(onDisk))
	})

	t.Run("builds public urls from the configured base", func(t *testing.T) {
		t.Parallel()
		disk, err := local.New(local.Options{Root: t.TempDir(), URL: "https://cdn.example.com/assets/"})
		require.NoError(t, err)

		link, err := disk.URL("logos/acme co.png")

		require.NoError(t, err)
		assert.Equal(t, "https://cdn.example.com/assets/logos/acme%20co.png", link)
	})

	t.Run("has no public or signed urls unless configured", func(t *testing.T) {
		t.Parallel()
		disk, err := local.New(local.Options{Root: t.TempDir()})
		require.NoError(t, err)

		_, err = disk.URL("a.png")
		require.ErrorIs(t, err, filesystem.ErrNoPublicURL)
		_, err = disk.TemporaryURL(t.Context(), "a.png", 0)
		require.ErrorIs(t, err, filesystem.ErrUnsupported)
	})

	t.Run("rejects bad configuration", func(t *testing.T) {
		t.Parallel()

		_, err := local.New(local.Options{})
		require.ErrorIs(t, err, local.ErrRootRequired)

		_, err = local.New(local.Options{Root: t.TempDir(), SignedURLBase: "://bad", SigningKey: "k"})
		require.ErrorContains(t, err, "signed_url_base")

		_, err = local.Factory("x", config.Section{"root": []int{1}})
		require.Error(t, err)
	})

	t.Run("fails when the root cannot be created", func(t *testing.T) {
		t.Parallel()
		file := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(file, nil, 0o600))

		_, err := local.New(local.Options{Root: filepath.Join(file, "sub")})
		require.ErrorContains(t, err, "open local disk")

		_, err = local.New(local.Options{Root: file})
		require.ErrorContains(t, err, "open local disk")
	})

	t.Run("closes", func(t *testing.T) {
		t.Parallel()
		disk, err := local.New(local.Options{Root: t.TempDir()})
		require.NoError(t, err)

		require.NoError(t, disk.Close())
	})
}

func TestStore(t *testing.T) {
	t.Parallel()

	newRoot := func(t *testing.T) (*filesystem.Disk, string) {
		t.Helper()
		root := t.TempDir()
		disk, err := local.New(local.Options{Root: root})
		require.NoError(t, err)

		return disk, root
	}

	t.Run("refuses the reserved attributes suffix", func(t *testing.T) {
		t.Parallel()
		disk, _ := newRoot(t)

		require.ErrorIs(t, disk.Put(t.Context(), "a.attrs", nil, filesystem.PutOptions{}), filesystem.ErrInvalidPath)
		_, err := disk.Get(t.Context(), "a.attrs")
		require.ErrorIs(t, err, filesystem.ErrInvalidPath)
		_, err = disk.Stat(t.Context(), "a.attrs")
		require.ErrorIs(t, err, filesystem.ErrInvalidPath)
	})

	t.Run("lists in path order and hides attributes", func(t *testing.T) {
		t.Parallel()
		disk, _ := newRoot(t)
		for _, path := range []string{"a/b", "a-c", "a.d", "ab/x", "abc"} {
			require.NoError(t, disk.Put(t.Context(), path, []byte("x"), filesystem.PutOptions{}))
		}

		var paths []string
		for entry, err := range disk.List(t.Context(), "a") {
			require.NoError(t, err)
			paths = append(paths, entry.Path)
		}

		assert.Equal(t, []string{"a-c", "a.d", "a/b", "ab/x", "abc"}, paths)
	})

	t.Run("treats folders and paths through files as missing", func(t *testing.T) {
		t.Parallel()
		disk, _ := newRoot(t)
		require.NoError(t, disk.Put(t.Context(), "dir/file", []byte("x"), filesystem.PutOptions{}))

		for _, path := range []string{"dir", "dir/file/inner"} {
			_, err := disk.Get(t.Context(), path)
			require.ErrorIs(t, err, filesystem.ErrNotFound, path)
			_, err = disk.Stat(t.Context(), path)
			require.ErrorIs(t, err, filesystem.ErrNotFound, path)
			require.NoError(t, disk.Delete(t.Context(), path), path)
		}

		assert.Empty(t, list(t, disk, "dir/file/"))
		require.Error(t, disk.Put(t.Context(), "dir/file/inner", []byte("x"), filesystem.PutOptions{}))
		require.Error(t, disk.Put(t.Context(), "dir", []byte("x"), filesystem.PutOptions{}))
	})

	t.Run("never follows links out of the root", func(t *testing.T) {
		t.Parallel()
		disk, root := newRoot(t)
		outside := filepath.Join(t.TempDir(), "secret")
		require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
		require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

		_, err := disk.Get(t.Context(), "link")

		require.Error(t, err)
		assert.Empty(t, list(t, disk, ""))
	})

	t.Run("reads files written without attributes", func(t *testing.T) {
		t.Parallel()
		disk, root := newRoot(t)
		require.NoError(t, os.WriteFile(filepath.Join(root, "plain"), []byte("x"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(root, "broken"), []byte("x"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(root, "broken.attrs"), []byte("{"), 0o600))

		attrs, err := disk.Stat(t.Context(), "plain")
		require.NoError(t, err)
		assert.Equal(t, "application/octet-stream", attrs.ContentType)

		_, err = disk.Stat(t.Context(), "broken")
		require.ErrorContains(t, err, "read attributes")
	})

	t.Run("stops listing when the context ends", func(t *testing.T) {
		t.Parallel()
		disk, _ := newRoot(t)
		require.NoError(t, disk.Put(t.Context(), "a", []byte("x"), filesystem.PutOptions{}))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		for _, err := range disk.List(ctx, "") {
			require.ErrorIs(t, err, context.Canceled)
		}

		require.ErrorIs(t, disk.Put(ctx, "b", []byte("x"), filesystem.PutOptions{}), context.Canceled)
		exists, err := disk.Exists(t.Context(), "b")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestVerify(t *testing.T) {
	t.Parallel()

	const key = "test-signing-key"

	disk, err := local.New(local.Options{
		Root:          t.TempDir(),
		SignedURLBase: "https://app.example.com/files/",
		SigningKey:    key,
	})
	require.NoError(t, err)

	request := func(t *testing.T, method, raw string) (*http.Request, string) {
		t.Helper()
		u, err := url.Parse(raw)
		require.NoError(t, err)

		return httptest.NewRequestWithContext(t.Context(), method, raw, nil), strings.TrimPrefix(u.Path, "/files/")
	}

	now := clock.System().Now()

	t.Run("accepts the signed request until it expires", func(t *testing.T) {
		t.Parallel()
		read, err := disk.TemporaryURL(t.Context(), "docs/q1 final.pdf", time.Minute)
		require.NoError(t, err)
		r, path := request(t, http.MethodGet, read)

		assert.Equal(t, "docs/q1 final.pdf", path)
		require.NoError(t, local.Verify(r, path, key, now))
		r.Method = http.MethodHead
		require.NoError(t, local.Verify(r, path, key, now))
		require.ErrorIs(t, local.Verify(r, path, key, now.Add(2*time.Minute)), local.ErrExpired)
	})

	t.Run("lasts an hour when no ttl is given", func(t *testing.T) {
		t.Parallel()
		read, err := disk.TemporaryURL(t.Context(), "a", 0)
		require.NoError(t, err)
		r, path := request(t, http.MethodGet, read)

		require.NoError(t, local.Verify(r, path, key, now.Add(59*time.Minute)))
		require.ErrorIs(t, local.Verify(r, path, key, now.Add(61*time.Minute)), local.ErrExpired)
	})

	t.Run("rejects a changed request", func(t *testing.T) {
		t.Parallel()
		read, err := disk.TemporaryURL(t.Context(), "a", time.Minute)
		require.NoError(t, err)

		r, path := request(t, http.MethodGet, read)
		require.ErrorIs(t, local.Verify(r, "b", key, now), local.ErrInvalidSignature)
		require.ErrorIs(t, local.Verify(r, path, "other-key", now), local.ErrInvalidSignature)

		r, path = request(t, http.MethodPut, read)
		require.ErrorIs(t, local.Verify(r, path, key, now), local.ErrInvalidSignature)

		for _, query := range []string{"expires=x", "expires=1&signature=zz", "expires=1"} {
			r, path = request(t, http.MethodGet, "https://app.example.com/files/a?"+query)
			require.ErrorIs(t, local.Verify(r, path, key, now), local.ErrInvalidSignature, query)
		}
	})

	t.Run("pins the content type of an upload", func(t *testing.T) {
		t.Parallel()
		upload, err := disk.TemporaryUploadURL(t.Context(), "in.pdf", filesystem.UploadURLOptions{
			TTL:         time.Minute,
			ContentType: "application/pdf",
		})
		require.NoError(t, err)
		r, path := request(t, http.MethodPut, upload)

		require.ErrorIs(t, local.Verify(r, path, key, now), local.ErrInvalidSignature)
		r.Header.Set("Content-Type", "application/pdf")
		require.NoError(t, local.Verify(r, path, key, now))
		r.Method = http.MethodGet
		require.ErrorIs(t, local.Verify(r, path, key, now), local.ErrInvalidSignature)
	})

	t.Run("signs uploads without a content type", func(t *testing.T) {
		t.Parallel()
		upload, err := disk.TemporaryUploadURL(t.Context(), "in.bin", filesystem.UploadURLOptions{TTL: time.Minute})
		require.NoError(t, err)
		r, path := request(t, http.MethodPut, upload)
		r.Header.Set("Content-Type", "image/png")

		require.NoError(t, local.Verify(r, path, key, now))
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
