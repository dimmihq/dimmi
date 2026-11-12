package s3_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/filesystem/drivers/s3"
	"github.com/dimmihq/dimmi/framework/filesystem/drivertest"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func section(t *testing.T) config.Section {
	t.Helper()

	return config.Section{
		"driver":           "s3",
		"bucket":           testkit.NewBucket(t),
		"force_path_style": true,
		"url":              "https://cdn.example.com",
	}
}

/*
zeros is an endless reader of zero bytes.
*/
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)

	return len(p), nil
}

func newDisk(t *testing.T) filesystem.Filesystem {
	t.Helper()

	disk, err := s3.Factory(t.Context())("attachments", section(t))
	require.NoError(t, err)

	return disk
}

func TestDisk(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newDisk)
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("builds public urls from the configured base", func(t *testing.T) {
		t.Parallel()

		link, err := newDisk(t).URL("a/b.png")

		require.NoError(t, err)
		assert.Equal(t, "https://cdn.example.com/a/b.png", link)
	})

	t.Run("signs the pinned content type into upload urls", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
		require.NoError(t, err)
		upload, err := disk.TemporaryUploadURL(t.Context(), "in/report.pdf", filesystem.UploadURLOptions{
			TTL:         time.Minute,
			ContentType: "application/pdf",
		})
		require.NoError(t, err)

		signed, err := url.Parse(upload)
		require.NoError(t, err)
		assert.Contains(t, signed.Query().Get("X-Amz-SignedHeaders"), "content-type")

		right, err := client.New().
			WithHeader("Content-Type", "application/pdf").
			Put(t.Context(), upload, []byte("%PDF"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, right.Status)

		read, err := disk.TemporaryURL(t.Context(), "in/report.pdf", 0)
		require.NoError(t, err)
		got, err := client.New().Get(t.Context(), read)
		require.NoError(t, err)
		assert.Equal(t, "%PDF", string(got.Body))
		assert.Equal(t, "application/pdf", got.Header.Get("Content-Type"))
	})

	t.Run("signs links for the public endpoint browsers reach", func(t *testing.T) {
		t.Parallel()
		opts := section(t)
		opts["public_endpoint"] = "https://files.example.test"
		disk, err := s3.Factory(t.Context())("attachments", opts)
		require.NoError(t, err)

		read, err := disk.TemporaryURL(t.Context(), "in/report.pdf", time.Minute)
		require.NoError(t, err)
		signed, err := url.Parse(read)
		require.NoError(t, err)
		assert.Equal(t, "files.example.test", signed.Host)
		assert.Equal(t, "/"+opts.String("bucket")+"/in/report.pdf", signed.Path)

		require.NoError(t, disk.Put(t.Context(), "in/kept.txt", []byte("kept"), filesystem.PutOptions{}))
		got, err := disk.Get(t.Context(), "in/kept.txt")
		require.NoError(t, err)
		assert.Equal(t, "kept", string(got))
	})

	t.Run("streams large files in parts", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		body := bytes.Repeat([]byte("0123456789abcdef"), 2<<20)

		require.NoError(t, disk.PutStream(t.Context(), "large.bin", bytes.NewReader(body), filesystem.PutOptions{}))

		attrs, err := disk.Stat(t.Context(), "large.bin")
		require.NoError(t, err)
		assert.Equal(t, int64(len(body)), attrs.Size)
		got, err := disk.Get(t.Context(), "large.bin")
		require.NoError(t, err)
		assert.Equal(t, body, got)
	})

	t.Run("leaves no object when a large stream fails", func(t *testing.T) {
		t.Parallel()
		disk := newDisk(t)
		errRead := errors.New("read failed")
		body := io.MultiReader(io.LimitReader(zeros{}, 20<<20), iotest.ErrReader(errRead))

		err := disk.PutStream(t.Context(), "large.bin", body, filesystem.PutOptions{})

		require.ErrorIs(t, err, errRead)
		exists, err := disk.Exists(t.Context(), "large.bin")
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("requires a bucket", func(t *testing.T) {
		t.Parallel()

		_, err := s3.New(t.Context(), s3.Options{})

		require.ErrorIs(t, err, s3.ErrBucketRequired)
	})

	t.Run("rejects a malformed section", func(t *testing.T) {
		t.Parallel()

		_, err := s3.Factory(t.Context())("x", config.Section{"bucket": []int{1}})

		require.Error(t, err)
	})

	t.Run("reports errors from the store", func(t *testing.T) {
		t.Parallel()
		disk, err := s3.New(t.Context(), s3.Options{
			Bucket:          "does-not-exist",
			Endpoint:        "http://127.0.0.1:1",
			ForcePathStyle:  true,
			AccessKeyID:     "x",
			SecretAccessKey: "y",
		})
		require.NoError(t, err)

		_, err = disk.Exists(t.Context(), "a")
		require.ErrorContains(t, err, `check "a"`)
		for _, err := range disk.List(t.Context(), "") {
			require.ErrorContains(t, err, "list")
		}
	})
}
