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
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/httpclient"
)

/*
stored is a file to put on the app's local disk: where, its type and its
contents.
*/
type stored struct{ path, contentType, body string }

/*
linkOn stores the file on the app's local disk and returns a signed link
to it, pointed at the running app.
*/
func linkOn(t *testing.T, app *foundation.Application, file stored) string {
	t.Helper()

	disk, err := app.Storage.Default()
	require.NoError(t, err)
	require.NoError(t, disk.Put(t.Context(), file.path, []byte(file.body), filesystem.PutOptions{
		ContentType: file.contentType,
	}))

	link, err := disk.TemporaryURL(t.Context(), file.path, time.Minute)
	require.NoError(t, err)

	address, err := url.Parse(link)
	require.NoError(t, err)

	return "http://" + app.Addr() + address.RequestURI()
}

func TestLocalFileLinks(t *testing.T) {
	t.Parallel()

	t.Run("serves a picture in place from a signed link", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))

		resp := mustGet(t, linkOn(t, app, stored{"a/shot.png", "image/png", "png-bytes"}))

		assert.Equal(t, http.StatusOK, resp.Status)
		assert.Equal(t, "png-bytes", string(resp.Body))
		assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
		assert.Empty(t, resp.Header.Get("Content-Disposition"))
		assert.Equal(t, "cross-origin", resp.Header.Get("Cross-Origin-Resource-Policy"))
	})

	t.Run("downloads anything that is not a picture", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))

		resp := mustGet(t, linkOn(t, app, stored{"a/page.html", "text/html", "<script>x</script>"}))

		assert.Equal(t, "attachment", resp.Header.Get("Content-Disposition"))
		assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
		assert.Contains(t, resp.Header.Get("Content-Security-Policy"), "sandbox")
	})

	t.Run("finds nothing for an altered, unsigned or unknown link", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))
		link := linkOn(t, app, stored{"a/shot.png", "image/png", "png-bytes"})
		base := "http://" + app.Addr()

		assert.Equal(t, http.StatusNotFound, mustGet(t, strings.Replace(link, "a/shot.png", "a/other.png", 1)).Status)
		assert.Equal(t, http.StatusNotFound, mustGet(t, strings.Split(link, "?")[0]).Status)
		assert.Equal(t, http.StatusNotFound, mustGet(t, base+"/files/nowhere/a/shot.png").Status)
	})

	t.Run("finds nothing when the file is gone", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))
		link := linkOn(t, app, stored{"a/gone.png", "image/png", "png-bytes"})

		disk, err := app.Storage.Default()
		require.NoError(t, err)
		require.NoError(t, disk.Delete(t.Context(), "a/gone.png"))

		assert.Equal(t, http.StatusNotFound, mustGet(t, link).Status)
	})

	t.Run("takes an upload to a signed upload link, with the type it was signed for", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))
		disk, err := app.Storage.Default()
		require.NoError(t, err)

		link, err := disk.TemporaryUploadURL(t.Context(), "up/shot.png", filesystem.UploadURLOptions{
			TTL: time.Minute, ContentType: "image/png",
		})
		require.NoError(t, err)
		address, err := url.Parse(link)
		require.NoError(t, err)
		target := "http://" + app.Addr() + address.RequestURI()

		client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true, Timeout: 5 * time.Second})
		require.NoError(t, err)

		wrong, err := client.New().WithHeader("Content-Type", "text/html").Put(t.Context(), target, "x")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, wrong.Status)

		right, err := client.New().WithHeader("Content-Type", "image/png").Put(t.Context(), target, "png")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, right.Status)

		stored, err := disk.Get(t.Context(), "up/shot.png")
		require.NoError(t, err)
		assert.Equal(t, "png", string(stored))
	})

	t.Run("answers a request for part of a file", func(t *testing.T) {
		t.Parallel()
		app := newApp(t, options(environ(t)))
		require.NoError(t, app.Start(t.Context()))
		link := linkOn(t, app, stored{"a/voice.webm", "audio/webm", "0123456789"})

		client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true, Timeout: 5 * time.Second})
		require.NoError(t, err)
		resp, err := client.New().WithHeader("Range", "bytes=2-4").Get(t.Context(), link)
		require.NoError(t, err)

		assert.Equal(t, http.StatusPartialContent, resp.Status)
		assert.Equal(t, "234", string(resp.Body))
		assert.Equal(t, "audio/webm", resp.Header.Get("Content-Type"))
	})
}
