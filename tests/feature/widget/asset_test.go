package widget_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/support/widgetassets"
	"github.com/dimmihq/dimmi/tests"
)

const widgetManifest = `{
  "version": "abc",
  "entry": "core-abc.js",
  "files": [{"name": "core-abc.js", "size": 9, "integrity": "sha384-current"}],
  "previous": ["core-old.js"]
}`

const loader = `(function(){var b=__DIMMI_BUILD__;load(b.core)})();`

/*
build returns a widget build with a current core and the previous
release's core still in it, changed by the edits given.
*/
func build(edits ...func(fstest.MapFS)) fstest.MapFS {
	fsys := fstest.MapFS{
		"manifest.json":         &fstest.MapFile{Data: []byte(widgetManifest)},
		"loader.js":             &fstest.MapFile{Data: []byte(loader)},
		"assets/core-abc.js":    &fstest.MapFile{Data: []byte("core();")},
		"assets/core-abc.js.gz": &fstest.MapFile{Data: []byte("gz")},
		"assets/core-abc.js.br": &fstest.MapFile{Data: []byte("b")},
		"assets/core-old.js":    &fstest.MapFile{Data: []byte("old();")},
		"assets/core-old.js.br": &fstest.MapFile{Data: []byte("o")},
		"assets/core-old.js.gz": &fstest.MapFile{Data: []byte("og")},
	}
	for _, edit := range edits {
		edit(fsys)
	}

	return fsys
}

func withWidget(fsys fstest.MapFS) tests.Option {
	return tests.Before(func(a *app.App) { a.Widget = fsys })
}

/*
assertPublic checks the headers every widget file carries, so any
customer's page may load it.
*/
func assertPublic(t *testing.T, h fhttp.Header) {
	t.Helper()

	assert.Equal(t, "*", h.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "cross-origin", h.Get("Cross-Origin-Resource-Policy"))
	assert.Equal(t, "*", h.Get("Timing-Allow-Origin"))
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
}

func gunzip(t *testing.T, data []byte) string {
	t.Helper()

	r, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)

	plain, err := io.ReadAll(r)
	require.NoError(t, err)

	return string(plain)
}

func TestWidgetLoader(t *testing.T) {
	t.Parallel()

	t.Run("serves the loader filled in with the current core", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(build()))

		res := tc.Client().Send(fhttp.MethodGet, "/widget/v1/loader.js", nil).AssertOK()

		assert.Equal(t,
			`(function(){var b={"assets":"","core":"core-abc.js","integrity":"sha384-current"};load(b.core)})();`,
			string(res.Body))
		assert.Equal(t, "text/javascript; charset=utf-8", res.Header.Get("Content-Type"))
		assert.Equal(t, "public, max-age=300, stale-while-revalidate=86400", res.Header.Get("Cache-Control"))
		assert.NotEmpty(t, res.Header.Get("ETag"))
		assertPublic(t, res.Header)
	})

	t.Run("points the loader at the assets host when one is configured", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(build()), tests.WithEnv(
			"DIMMI_WIDGET__ASSETS_BASE_URL=https://cdn.example.com/",
			"DIMMI_WIDGET__LOADER_MAX_AGE=1m",
		))

		res := tc.Client().Send(fhttp.MethodGet, "/widget/v1/loader.js", nil).AssertOK()

		assert.Contains(t, string(res.Body), `"assets":"https://cdn.example.com/widget/assets/"`)
		assert.Equal(t, "public, max-age=60, stale-while-revalidate=86400", res.Header.Get("Cache-Control"))
	})

	t.Run("answers 304 when the client already has this loader", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(build()))
		etag := tc.Client().Send(fhttp.MethodGet, "/widget/v1/loader.js", nil).AssertOK().Header.Get("ETag")

		res := tc.Client().WithHeader("If-None-Match", etag).Send(fhttp.MethodGet, "/widget/v1/loader.js", nil)

		res.AssertStatus(fhttp.StatusNotModified)
		assert.Empty(t, res.Body)
	})

	t.Run("sends the loader gzipped to clients that accept gzip", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(build()))

		res := tc.Client().WithHeader("Accept-Encoding", "gzip, deflate, br").
			Send(fhttp.MethodGet, "/widget/v1/loader.js", nil).AssertOK()

		assert.Equal(t, "gzip", res.Header.Get("Content-Encoding"))
		assert.Contains(t, res.Header.Values("Vary"), "Accept-Encoding")
		assert.Contains(t, gunzip(t, res.Body), `"core":"core-abc.js"`)
	})
}

func TestWidgetManifest(t *testing.T) {
	t.Parallel()

	t.Run("publishes the build's files, sizes and integrity hashes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(build()))

		res := tc.Client().Send(fhttp.MethodGet, "/widget/v1/manifest.json", nil).AssertOK()

		assert.JSONEq(t, widgetManifest, string(res.Body))
		assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
		assert.Equal(t, "public, max-age=60", res.Header.Get("Cache-Control"))
		assertPublic(t, res.Header)
	})
}

func TestWidgetAssetFiles(t *testing.T) {
	t.Parallel()

	t.Run("serves a hashed file in the smallest encoding the client takes, cached forever", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(build()))

		brotli := tc.Client().WithHeader("Accept-Encoding", "gzip, br").
			Send(fhttp.MethodGet, "/widget/assets/core-abc.js", nil).AssertOK()
		gzipped := tc.Client().WithHeader("Accept-Encoding", "gzip").
			Send(fhttp.MethodGet, "/widget/assets/core-abc.js", nil).AssertOK()
		plain := tc.Client().Send(fhttp.MethodGet, "/widget/assets/core-abc.js", nil).AssertOK()

		assert.Equal(t, "br", brotli.Header.Get("Content-Encoding"))
		assert.Equal(t, "b", string(brotli.Body))
		assert.Equal(t, "gzip", gzipped.Header.Get("Content-Encoding"))
		assert.Equal(t, "core();", string(plain.Body))
		assert.Equal(t, "public, max-age=31536000, immutable", plain.Header.Get("Cache-Control"))
		assert.Contains(t, plain.Header.Get("Content-Type"), "javascript")
		assertPublic(t, plain.Header)
	})

	t.Run("keeps serving the previous release's files after a deploy", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(build()))

		res := tc.Client().Send(fhttp.MethodGet, "/widget/assets/core-old.js", nil).AssertOK()

		assert.Equal(t, "old();", string(res.Body))
	})

	t.Run("answers 404 for a file the build does not have", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(build()))

		tc.Client().Send(fhttp.MethodGet, "/widget/assets/core-gone.js", nil).AssertNotFound()
	})

	t.Run("answers 404 for every widget file in a binary built without the widget", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withWidget(fstest.MapFS{}))

		paths := []string{"/widget/v1/loader.js", "/widget/v1/manifest.json", "/widget/assets/core-abc.js"}
		for _, path := range paths {
			res := tc.Client().Send(fhttp.MethodGet, path, nil).AssertNotFound()
			assertPublic(t, res.Header)
		}
	})
}

func TestWidgetBuildAtBoot(t *testing.T) {
	t.Parallel()

	broken := map[string]func(fstest.MapFS){
		"a manifest that does not parse": func(fsys fstest.MapFS) {
			fsys["manifest.json"] = &fstest.MapFile{Data: []byte("{")}
		},
		"a manifest whose core is not listed": func(fsys fstest.MapFS) {
			fsys["manifest.json"] = &fstest.MapFile{Data: []byte(`{"entry": "core-abc.js", "files": []}`)}
		},
		"no loader": func(fsys fstest.MapFS) {
			delete(fsys, "loader.js")
		},
		"a loader without its placeholder": func(fsys fstest.MapFS) {
			fsys["loader.js"] = &fstest.MapFile{Data: []byte("load();")}
		},
	}

	for name, edit := range broken {
		t.Run("refuses to boot with "+name, func(t *testing.T) {
			t.Parallel()

			_, _, err := tests.Boot(t, withWidget(build(edit)))

			require.ErrorIs(t, err, widgetassets.ErrBrokenBuild)
		})
	}
}
