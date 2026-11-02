package vite_test

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/view"
	"github.com/dimmihq/dimmi/framework/view/vite"
)

const manifest = `{
  "src/main.ts": {
    "file": "assets/main-4f2a.js",
    "src": "src/main.ts",
    "isEntry": true,
    "imports": ["_shared-9c1d.js"],
    "css": ["assets/main-77aa.css"]
  },
  "_shared-9c1d.js": {
    "file": "assets/shared-9c1d.js",
    "css": ["assets/shared-1b2c.css"]
  }
}`

/*
failingWriter refuses every write, as a connection the client closed
does.
*/
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func build(name string) fstest.MapFS {
	return fstest.MapFS{name: &fstest.MapFile{Data: []byte(manifest)}}
}

func TestManifest(t *testing.T) {
	t.Parallel()

	t.Run("reads the manifest where current vite writes it", func(t *testing.T) {
		t.Parallel()
		_, err := vite.Manifest(build(".vite/manifest.json"), "/build")
		require.NoError(t, err)
	})

	t.Run("reads the manifest where older vite wrote it", func(t *testing.T) {
		t.Parallel()
		_, err := vite.Manifest(build("manifest.json"), "/build")
		require.NoError(t, err)
	})

	t.Run("fails without a manifest", func(t *testing.T) {
		t.Parallel()
		_, err := vite.Manifest(fstest.MapFS{}, "/build")
		require.ErrorIs(t, err, vite.ErrNoManifest)
	})

	t.Run("fails on a manifest that is not json", func(t *testing.T) {
		t.Parallel()
		fsys := fstest.MapFS{"manifest.json": &fstest.MapFile{Data: []byte("{")}}
		_, err := vite.Manifest(fsys, "/build")
		require.ErrorContains(t, err, "parse vite manifest")
	})

	t.Run("fails when the manifest cannot be opened", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o000))
		_, err := vite.Manifest(os.DirFS(dir), "/build")
		require.ErrorIs(t, err, fs.ErrPermission)
	})
}

func TestTags(t *testing.T) {
	t.Parallel()

	assets, err := vite.Manifest(build(".vite/manifest.json"), "/build/")
	require.NoError(t, err)

	t.Run("writes stylesheets, preloads and the module script of the entry", func(t *testing.T) {
		t.Parallel()
		got, err := view.HTML(t.Context(), assets.Tags("src/main.ts"))
		require.NoError(t, err)
		assert.Equal(t, `<link rel="stylesheet" href="/build/assets/main-77aa.css">`+
			`<link rel="stylesheet" href="/build/assets/shared-1b2c.css">`+
			`<link rel="modulepreload" href="/build/assets/main-4f2a.js">`+
			`<link rel="modulepreload" href="/build/assets/shared-9c1d.js">`+
			`<script type="module" src="/build/assets/main-4f2a.js"></script>`, got)
	})

	t.Run("puts the request nonce on every tag", func(t *testing.T) {
		t.Parallel()
		ctx := view.WithNonce(t.Context(), `a"b`)
		got, err := view.HTML(ctx, assets.Tags("src/main.ts"))
		require.NoError(t, err)
		assert.Contains(t, got, `<script nonce="a&#34;b" type="module"`)
		assert.Contains(t, got, `<link nonce="a&#34;b" rel="stylesheet"`)
		assert.Contains(t, got, `<link nonce="a&#34;b" rel="modulepreload"`)
	})

	t.Run("returns the write error", func(t *testing.T) {
		t.Parallel()
		err := assets.Tags("src/main.ts").Render(t.Context(), failingWriter{})
		require.ErrorIs(t, err, io.ErrClosedPipe)
	})

	t.Run("fails for an entry the build does not have", func(t *testing.T) {
		t.Parallel()
		_, err := view.HTML(t.Context(), assets.Tags("src/missing.ts"))
		require.ErrorIs(t, err, vite.ErrUnknownEntry)
	})
}
