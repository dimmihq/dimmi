package foundation_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestAppLinkedStorage(t *testing.T) {
	t.Parallel()

	t.Run("serves a storage service's files under the app's own address", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t,
			"APP_FILESYSTEMS__DISKS__LOCAL__DRIVER=local",
			"APP_FILESYSTEMS__DISKS__LOCAL__ROOT="+t.TempDir(),
			"APP_FILESYSTEMS__DISKS__STORE__DRIVER=s3",
			"APP_FILESYSTEMS__DISKS__STORE__BUCKET="+testkit.NewBucket(t),
			"APP_FILESYSTEMS__DISKS__STORE__FORCE_PATH_STYLE=true",
			"APP_FILESYSTEMS__DISKS__STORE__LINKS=app",
		)))
		require.NoError(t, app.Start(t.Context()))
		disk, err := app.Storage.Disk("store")
		require.NoError(t, err)
		require.NoError(t, disk.Put(t.Context(), "a/0193.bin", []byte("%PDF"), filesystem.PutOptions{
			ContentType: "application/pdf",
		}))

		link, err := disk.DownloadURL(t.Context(), "a/0193.bin", filesystem.DownloadURLOptions{
			TTL: time.Minute, Filename: "receipt.pdf",
		})
		require.NoError(t, err)
		require.Contains(t, link, "/files/store/a/0193.bin")
		address, err := url.Parse(link)
		require.NoError(t, err)
		served := "http://" + app.Addr() + address.RequestURI()

		resp := mustGet(t, served)
		assert.Equal(t, http.StatusOK, resp.Status)
		assert.Equal(t, "%PDF", string(resp.Body))
		assert.Equal(t, "attachment; filename=receipt.pdf", resp.Header.Get("Content-Disposition"))
		assert.Equal(t, http.StatusNotFound, mustGet(t, strings.Replace(served, "0193", "0194", 1)).Status)
	})

	t.Run("takes an upload to the storage service through the app", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t,
			"APP_FILESYSTEMS__DISKS__LOCAL__DRIVER=local",
			"APP_FILESYSTEMS__DISKS__LOCAL__ROOT="+t.TempDir(),
			"APP_FILESYSTEMS__DISKS__STORE__DRIVER=s3",
			"APP_FILESYSTEMS__DISKS__STORE__BUCKET="+testkit.NewBucket(t),
			"APP_FILESYSTEMS__DISKS__STORE__FORCE_PATH_STYLE=true",
			"APP_FILESYSTEMS__DISKS__STORE__LINKS=app",
		)))
		require.NoError(t, app.Start(t.Context()))
		disk, err := app.Storage.Disk("store")
		require.NoError(t, err)

		upload, err := disk.TemporaryUploadURL(t.Context(), "in/note.txt", filesystem.UploadURLOptions{
			TTL: time.Minute, ContentType: "text/plain",
		})
		require.NoError(t, err)
		address, err := url.Parse(upload)
		require.NoError(t, err)
		client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
		require.NoError(t, err)

		sent, err := client.New().WithHeader("Content-Type", "text/plain").
			Put(t.Context(), "http://"+app.Addr()+address.RequestURI(), []byte("hello"))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, sent.Status)

		kept, err := disk.Get(t.Context(), "in/note.txt")
		require.NoError(t, err)
		assert.Equal(t, "hello", string(kept))
	})
}
