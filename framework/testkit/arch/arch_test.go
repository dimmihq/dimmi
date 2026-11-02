package arch_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit/arch"
)

/*
recorder records failures instead of failing the test. Fatalf stops the
goroutine it runs in, as the real one does.
*/
type recorder struct {
	testing.TB
	mu       sync.Mutex
	failures []string
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	runtime.Goexit()
}

/*
record runs fn with a recorder in place of the test and returns the
failures it recorded.
*/
func record(t *testing.T, fn func(tb *recorder)) []string {
	t.Helper()

	rec := &recorder{TB: t}
	done := make(chan struct{})

	go func() {
		defer close(done)
		fn(rec)
	}()

	<-done

	return rec.failures
}

/*
module writes a module with the files into a temporary directory and
returns its root.
*/
func module(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	files["go.mod"] = "module example.com/shop\n\ngo 1.26\n"

	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	}

	return root
}

/*
shop is a module with a core package that imports the app package and
the support package, the latter only from its tests, plus directories
the go command ignores.
*/
func shop(t *testing.T) string {
	t.Helper()

	return module(t, map[string]string{
		"main.go": "package main\n\nfunc main() {}\n",
		"core/core.go": "package core\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/shop/app\"\n)\n\n" +
			"var _ = fmt.Sprint(app.Name)\n",
		"core/core_test.go": "package core_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/shop/core\"\n" +
			"\t\"example.com/shop/support\"\n)\n\nfunc TestCore(*testing.T) { _ = support.X }\n\nvar _ = core.X\n",
		"core/x.go":               "package core\n\nconst X = 1\n",
		"core/testdata/notes.txt": "notes",
		"app/app.go":              "package app\n\nconst Name = \"app\"\n",
		"support/support.go":      "package support\n\nconst X = 1\n",
		"docs/readme.txt":         "no go here",
		"_scratch/x.go":           "package broken!",
		".hidden/x.go":            "package broken!",
		"vendor/x/x.go":           "package broken!",
		"nested/go.mod":           "module example.com/nested\n",
		"nested/x.go":             "package broken!",
	})
}

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run("loads the packages the go command builds", func(t *testing.T) {
		t.Parallel()

		var paths []string
		for _, p := range arch.LoadDir(t, shop(t)).Packages() {
			paths = append(paths, p.Path)
		}

		assert.ElementsMatch(t, []string{".", "app", "core", "support"}, paths)
	})

	t.Run("lists the files of a package and its testdata", func(t *testing.T) {
		t.Parallel()

		for _, p := range arch.LoadDir(t, shop(t)).Packages() {
			if p.Path == "core" {
				want := []string{"core/core.go", "core/core_test.go", "core/x.go", "core/testdata/notes.txt"}
				assert.ElementsMatch(t, want, p.Files)
			}
		}
	})

	t.Run("finds the module above the working directory", func(t *testing.T) {
		t.Parallel()

		m := arch.Load(t, "github.com/dimmihq/dimmi")

		assert.NotEmpty(t, m.Packages())
	})

	t.Run("fails for a module it cannot find or read", func(t *testing.T) {
		t.Parallel()

		unknown := record(t, func(tb *recorder) { arch.Load(tb, "example.com/unknown") })
		missing := record(t, func(tb *recorder) { arch.LoadDir(tb, t.TempDir()) })
		broken := record(t, func(tb *recorder) {
			arch.LoadDir(tb, module(t, map[string]string{"a/a.go": "package a\n", "a/b.go": "package b\n"}))
		})

		assert.Contains(t, unknown[0], "no go.mod declares example.com/unknown")
		assert.Contains(t, missing[0], "no go.mod declares a module")
		assert.Contains(t, broken[0], "load module example.com/shop")
	})
}

func TestImports(t *testing.T) {
	t.Parallel()

	m := arch.LoadDir(t, shop(t))

	t.Run("passes packages that keep to the rules", func(t *testing.T) {
		t.Parallel()

		m.Expect("app", "support").NotToImport("core/...").ToOnlyImport(arch.Std)
		m.Expect("core").IgnoringTests().NotToImport("support")
		m.Expect("core").ToOnlyImport(arch.Std, "app", "support")
		m.Expect("...").Except("core", ".").NotToImport("github.com/...")
	})

	t.Run("reports every import that breaks a rule", func(t *testing.T) {
		t.Parallel()

		failures := record(t, func(tb *recorder) {
			m := arch.LoadDir(tb, shop(t))
			m.Expect("core").NotToImport("app", "sup*")
			m.Expect("core/...").IgnoringTests().ToOnlyImport("example.com/shop/app")
		})

		assert.Equal(t, []string{
			`arch: core imports example.com/shop/app, which matches "app"`,
			`arch: core imports example.com/shop/support, which matches "sup*"`,
			"arch: core imports fmt, which is not one of example.com/shop/app",
		}, failures)
	})

	t.Run("fails for a pattern that matches no package", func(t *testing.T) {
		t.Parallel()

		failures := record(t, func(tb *recorder) { arch.LoadDir(tb, shop(t)).Expect("nothing/...") })

		assert.Equal(t, []string{`arch: no package matches "nothing/..."`}, failures)
	})
}

func TestWords(t *testing.T) {
	t.Parallel()

	root := module(t, map[string]string{
		"core/core.go": "/*\nPackage core.\n\nIt mentions bananas.\n*/\npackage core\n\nimport (\n" +
			"\tpeel \"example.com/shop/core/banana_peel\"\n\t\"example.com/shop/lib\"\n)\n\n" +
			"const Fruit = \"BananaSplit\"\n\nvar _ = lib.BananaBread\n\nvar _ = peel.X\n\n" +
			"var _ = \"see example.com/shop/banana\"\n\nfunc handleBananaRequest() {}\n\nvar _ = `raw`\n",
		"core/banana_test.go":      "package core\n\nconst borders, Turban, BAN, xbanana = 1, 2, 3, 4\n",
		"core/testdata/banana.txt": "x",
		"core/banana_peel/peel.go": "package bananapeel\n\nconst X = 1\n",
		"lib/lib.go":               "package lib\n\nconst BananaBread = 1\n",
		"clean/clean.go": "package clean\n\n// The word is only in the module path: example.com/shop/banana.\n" +
			"const X = 1\n",
		"clean/broken/go_broken.go": "package broken\n\nfunc {\n",
	})
	m := arch.LoadDir(t, root)

	t.Run("finds stems in identifiers, strings, comments and file names", func(t *testing.T) {
		t.Parallel()

		failures := record(t, func(tb *recorder) {
			arch.LoadDir(tb, root).Expect("core").NotToContainWords("Ban-ana")
		})

		assert.ElementsMatch(t, []string{
			`arch: core/banana_test.go: file name "banana_test" contains "banana"`,
			`arch: core/testdata/banana.txt: file name "banana" contains "banana"`,
			`arch: core/core.go:4: comment "bananas" contains "banana"`,
			`arch: core/core.go:13: string "BananaSplit" contains "banana"`,
			`arch: core/core.go:21: identifier "handleBananaRequest" contains "banana"`,
		}, failures)
	})

	t.Run("leaves out test files when told to", func(t *testing.T) {
		t.Parallel()

		failures := record(t, func(tb *recorder) {
			arch.LoadDir(tb, root).Expect("core").IgnoringTests().NotToContainWords("banana")
		})

		assert.Len(t, failures, 4)
	})

	t.Run("reads the module path and other packages' names as no words", func(t *testing.T) {
		t.Parallel()

		m.Expect("clean").NotToContainWords("banana")
	})

	t.Run("reports a file it cannot parse", func(t *testing.T) {
		t.Parallel()

		failures := record(t, func(tb *recorder) {
			arch.LoadDir(tb, root).Expect("clean/broken").NotToContainWords("banana")
		})

		assert.Contains(t, failures[0], "arch: parse clean/broken/go_broken.go")
	})
}
