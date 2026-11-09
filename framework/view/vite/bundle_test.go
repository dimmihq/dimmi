package vite_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/view"
	"github.com/dimmihq/dimmi/framework/view/vite"
)

/*
hotFile writes the dev server address to a fresh hot file and returns its
path.
*/
func hotFile(t *testing.T, address string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "hot")
	require.NoError(t, os.WriteFile(path, []byte(address), 0o600))

	return path
}

func TestBundle(t *testing.T) {
	t.Parallel()

	t.Run("loads the production build while no dev server runs", func(t *testing.T) {
		t.Parallel()
		bundle := vite.New(vite.Options{
			Build: build(".vite/manifest.json"), Base: "/build", HotFile: filepath.Join(t.TempDir(), "hot"),
		})

		got, err := view.HTML(t.Context(), bundle.Tags("src/main.ts"))

		require.NoError(t, err)
		assert.Contains(t, got, `<script type="module" src="/build/assets/main-4f2a.js"></script>`)
		_, running := bundle.DevServer()
		assert.False(t, running)
	})

	t.Run("loads the client and the entry from the running dev server, with the nonce", func(t *testing.T) {
		t.Parallel()
		bundle := vite.New(vite.Options{HotFile: hotFile(t, "http://localhost:5173/\n")})
		ctx := view.WithNonce(t.Context(), "n0nce")

		got, err := view.HTML(ctx, bundle.Tags("/src/main.ts"))

		require.NoError(t, err)
		assert.Equal(t, `<script nonce="n0nce" type="module" src="http://localhost:5173/@vite/client"></script>`+
			`<script nonce="n0nce" type="module" src="http://localhost:5173/src/main.ts"></script>`, got)
		server, running := bundle.DevServer()
		assert.True(t, running)
		assert.Equal(t, "http://localhost:5173", server)
	})

	t.Run("writes several entries from the production build in order and fails on an unknown one", func(t *testing.T) {
		t.Parallel()
		bundle := vite.New(vite.Options{Build: build(".vite/manifest.json"), Base: "/build"})

		got, err := view.HTML(t.Context(), bundle.Tags("src/site.css", "src/main.ts"))

		require.NoError(t, err)
		assert.Less(t, strings.Index(got, `rel="stylesheet" href="/build/assets/site-`),
			strings.Index(got, `src="/build/assets/main-4f2a.js"`))
		_, err = view.HTML(t.Context(), bundle.Tags("src/main.ts", "src/missing.ts"))
		require.ErrorIs(t, err, vite.ErrUnknownEntry)
	})

	t.Run("links a stylesheet entry from the dev server before the scripts that follow it", func(t *testing.T) {
		t.Parallel()
		bundle := vite.New(vite.Options{HotFile: hotFile(t, "http://localhost:5173")})

		got, err := view.HTML(t.Context(), bundle.Tags("src/site.css", "src/main.ts"))

		require.NoError(t, err)
		assert.Equal(t, `<script type="module" src="http://localhost:5173/@vite/client"></script>`+
			`<link rel="stylesheet" href="http://localhost:5173/src/site.css">`+
			`<script type="module" src="http://localhost:5173/src/main.ts"></script>`, got)
	})

	t.Run("installs the fast refresh runtime first for react", func(t *testing.T) {
		t.Parallel()
		bundle := vite.New(vite.Options{HotFile: hotFile(t, "http://localhost:5173"), React: true})

		got, err := view.HTML(t.Context(), bundle.Tags("src/main.tsx"))

		require.NoError(t, err)
		assert.Regexp(t, `^<script type="module">\s+import RefreshRuntime from 'http://localhost:5173/@react-refresh'`, got)
		assert.Contains(t, got, `src="http://localhost:5173/@vite/client"`)
	})

	t.Run("ignores an empty hot file and a bundle without one", func(t *testing.T) {
		t.Parallel()

		for _, opts := range []vite.Options{{HotFile: hotFile(t, " \n")}, {}} {
			_, running := vite.New(opts).DevServer()
			assert.False(t, running)
		}
	})

	t.Run("fails every page without a build, and keeps failing", func(t *testing.T) {
		t.Parallel()
		bundle := vite.New(vite.Options{Build: fstest.MapFS{}, Base: "/build"})

		for range 2 {
			_, err := view.HTML(t.Context(), bundle.Tags("src/main.ts"))
			require.ErrorIs(t, err, vite.ErrNoManifest)
		}
	})

	t.Run("returns the write error of the dev tags", func(t *testing.T) {
		t.Parallel()
		bundle := vite.New(vite.Options{HotFile: hotFile(t, "http://localhost:5173")})

		err := bundle.Tags("src/main.ts").Render(t.Context(), failingWriter{})

		require.ErrorIs(t, err, io.ErrClosedPipe)
	})
}
