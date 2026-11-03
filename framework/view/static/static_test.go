package static_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/view/static"
)

const script = "console.log('hello from a script that is long enough to be worth compressing');"

func gz(t *testing.T, s string) []byte {
	t.Helper()

	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, err := io.WriteString(w, s)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	return b.Bytes()
}

func assets(t *testing.T) fstest.MapFS {
	t.Helper()

	return fstest.MapFS{
		"app.js":          &fstest.MapFile{Data: []byte(script)},
		"app.js.gz":       &fstest.MapFile{Data: gz(t, script)},
		"only.css.gz":     &fstest.MapFile{Data: gz(t, "body{color:red}")},
		"docs/index.html": &fstest.MapFile{Data: []byte("<p>Docs</p>")},
	}
}

func get(t *testing.T, h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
	for k, v := range headers {
		r.Header.Set(k, v)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	return w
}

func TestHandler(t *testing.T) {
	t.Parallel()

	t.Run("serves a file with its content type", func(t *testing.T) {
		t.Parallel()
		w := get(t, static.Handler(assets(t), static.Options{}), "/app.js", nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, script, w.Body.String())
		assert.Equal(t, "application/javascript", w.Header().Get("Content-Type"))
		assert.Empty(t, w.Header().Get("Cache-Control"))
	})

	t.Run("serves the compressed copy to clients that accept gzip", func(t *testing.T) {
		t.Parallel()
		h := static.Handler(assets(t), static.Options{Precompressed: true})
		w := get(t, h, "/app.js", map[string]string{"Accept-Encoding": "gzip, br"})
		assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
		assert.Equal(t, gz(t, script), w.Body.Bytes())
	})

	t.Run("unpacks a compressed-only file for clients without gzip", func(t *testing.T) {
		t.Parallel()
		h := static.Handler(assets(t), static.Options{Precompressed: true})
		w := get(t, h, "/only.css", nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("Content-Encoding"))
		assert.Equal(t, "body{color:red}", w.Body.String())
	})

	t.Run("ignores compressed copies unless asked to use them", func(t *testing.T) {
		t.Parallel()
		h := static.Handler(assets(t), static.Options{})
		w := get(t, h, "/app.js", map[string]string{"Accept-Encoding": "gzip"})
		assert.Empty(t, w.Header().Get("Content-Encoding"))
		assert.Equal(t, script, w.Body.String())
		assert.Equal(t, http.StatusNotFound, get(t, h, "/only.css", nil).Code)
	})

	t.Run("marks files as immutable when asked", func(t *testing.T) {
		t.Parallel()
		h := static.Handler(assets(t), static.Options{Immutable: true})
		w := get(t, h, "/app.js", nil)
		assert.Equal(t, "public, max-age=31536000, immutable", w.Header().Get("Cache-Control"))
	})

	t.Run("serves files under the prefix they are mounted at", func(t *testing.T) {
		t.Parallel()
		h := static.Handler(assets(t), static.Options{Immutable: true, Prefix: "/build"})
		w := get(t, h, "/build/app.js", nil)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, script, w.Body.String())
		assert.Equal(t, http.StatusNotFound, get(t, h, "/app.js", nil).Code)
	})

	t.Run("does not mark a missing file as immutable", func(t *testing.T) {
		t.Parallel()
		h := static.Handler(assets(t), static.Options{Immutable: true})
		w := get(t, h, "/missing.js", nil)
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Empty(t, w.Header().Get("Cache-Control"))
	})

	t.Run("answers not modified for a matching etag", func(t *testing.T) {
		t.Parallel()
		h := static.Handler(assets(t), static.Options{})
		etag := get(t, h, "/app.js", nil).Header().Get("Etag")
		require.NotEmpty(t, etag)
		w := get(t, h, "/app.js", map[string]string{"If-None-Match": etag})
		assert.Equal(t, http.StatusNotModified, w.Code)
	})

	t.Run("serves index.html for a directory", func(t *testing.T) {
		t.Parallel()
		w := get(t, static.Handler(assets(t), static.Options{}), "/docs/", nil)
		assert.Equal(t, "<p>Docs</p>", w.Body.String())
	})

	t.Run("refuses methods other than get and head", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		static.Handler(assets(t), static.Options{}).
			ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/app.js", http.NoBody))
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}
