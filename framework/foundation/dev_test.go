//go:build dev

package foundation_test

import (
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
translatedFramework returns the framework's English messages and the
same files again as French, as an application that translated them all
ships.
*/
func translatedFramework(t *testing.T) fstest.MapFS {
	t.Helper()

	files := fstest.MapFS{}
	for _, dir := range []fs.FS{i18n.Messages(), validation.Messages(), fhttp.Messages()} {
		paths, err := fs.Glob(dir, "en/*.json")
		require.NoError(t, err)

		for _, name := range paths {
			data, err := fs.ReadFile(dir, name)
			require.NoError(t, err)
			files[name] = &fstest.MapFile{Data: data}
			files["fr/"+path.Base(name)] = &fstest.MapFile{Data: data}
		}
	}

	return files
}

func TestDevCommands(t *testing.T) {
	t.Parallel()

	t.Run("lists the generators and route linting in development builds", func(t *testing.T) {
		t.Parallel()

		code, stdout, _ := run(t, options(nil), "")

		require.Equal(t, console.ExitSuccess, code)
		for _, name := range []string{"make:action", "make:job", "make:migration", "route:lint", "lang:check"} {
			assert.Contains(t, stdout, name)
		}
	})

	t.Run("generates files under the project root", func(t *testing.T) {
		t.Parallel()
		opts := options(nil)
		opts.Module = "example.com/shop"
		opts.Root = t.TempDir()

		code, stdout, stderr := run(t, opts, "", "make:job", "SendReceipt")

		require.Equal(t, console.ExitSuccess, code, stderr)
		assert.Contains(t, stdout, "send_receipt")
		var generated []string
		require.NoError(t, filepath.WalkDir(opts.Root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.Contains(path, "send_receipt") {
				generated = append(generated, path)
			}

			return err
		}))
		require.NotEmpty(t, generated)
		contents, err := os.ReadFile(generated[0])
		require.NoError(t, err)
		assert.Contains(t, string(contents), "SendReceipt")
	})

	t.Run("lints routes against the rules", func(t *testing.T) {
		t.Parallel()
		routes := provider{register: func(a *foundation.Application) error {
			a.Router.Handle(http.MethodGet, "/v1/orders", "orders.index", http.NotFoundHandler())

			return nil
		}}
		opts := options(environ(t, "APP_MIGRATIONS__AUTO=false"), routes)
		opts.RouteRules.Prefixes = []string{"/v1"}

		code, stdout, _ := run(t, opts, "", "route:lint")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Every route follows the rules.")

		opts.RouteRules.VerbDenylist = []string{"orders"}
		code, _, stderr := run(t, opts, "", "route:lint")
		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "is a verb")
	})

	t.Run("checks every locale has every message and every message is used", func(t *testing.T) {
		t.Parallel()
		opts := options(nil)
		opts.Root = t.TempDir()
		code := "package app\n\nvar greeting = T(nil, \"messages.hello\", nil)\n"
		require.NoError(t, os.WriteFile(filepath.Join(opts.Root, "app.go"), []byte(code), 0o600))
		complete := translatedFramework(t)
		complete["en/messages.json"] = &fstest.MapFile{Data: []byte(`{"hello": "Hello"}`)}
		complete["fr/messages.json"] = &fstest.MapFile{Data: []byte(`{"hello": "Bonjour"}`)}
		opts.Language = complete

		status, stdout, stderr := run(t, opts, "", "lang:check")
		require.Equal(t, console.ExitSuccess, status, stderr)
		assert.Contains(t, stdout, "every message is used")

		incomplete := maps.Clone(complete)
		incomplete["en/messages.json"] = &fstest.MapFile{Data: []byte(`{"hello": "Hello", "bye": "Bye"}`)}
		delete(incomplete, "fr/validation.json")
		opts.Language = incomplete

		status, _, stderr = run(t, opts, "", "lang:check")
		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, stderr, "fr is missing messages.bye")
		assert.Contains(t, stderr, "fr is missing validation.required")
		assert.Contains(t, stderr, "messages.bye is not used")
	})

	t.Run("reports message files it cannot read", func(t *testing.T) {
		t.Parallel()
		opts := options(nil)
		opts.Root = t.TempDir()
		opts.Language = fstest.MapFS{"en/messages.json": {Data: []byte(`{`)}}

		status, _, stderr := run(t, opts, "", "lang:check")

		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, stderr, "parse en/messages.json")

		opts.Language = nil
		require.NoError(t, os.WriteFile(filepath.Join(opts.Root, "bad.go"), []byte("package"), 0o600))
		status, _, stderr = run(t, opts, "", "lang:check")
		assert.Equal(t, console.ExitFailure, status)
		assert.Contains(t, stderr, "parse bad.go")
	})

	t.Run("reports an application that cannot boot for linting", func(t *testing.T) {
		t.Parallel()

		code, _, _ := run(t, options(environ(t, "APP_REDIS__URL=redis://127.0.0.1:1")), "", "route:lint")

		assert.Equal(t, console.ExitFailure, code)
	})
}
