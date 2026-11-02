/*
|--------------------------------------------------------------------------
| Architecture Tests
|--------------------------------------------------------------------------
|
| The layering of a codebase only holds if something checks it. These
| helpers load every package of a module straight from its source files,
| with the build constraints of the current platform, and let a test
| state rules about them the way it states any other expectation: which
| packages may import which, and which words must never appear.
|
| Packages are named relative to the module, as in "framework/...": a
| "*" stands for one path element and "..." for any number, including
| none. Paths outside the module are written in full, and Std stands for
| the standard library.
|
*/

package arch

import (
	"errors"
	"fmt"
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/*
Module is a loaded module: its path, its root directory and its
packages.
*/
type Module struct {
	tb       testing.TB
	path     string
	root     string
	packages []*Package
}

/*
Package is one package of the module. Path is its import path relative
to the module and Dir its directory. Imports lists what its non-test
files import and TestImports what its test files import. Files lists
every file in its directory and in the testdata below it, relative to
the module root.
*/
type Package struct {
	Path        string
	Dir         string
	Imports     []string
	TestImports []string
	GoFiles     []string
	TestGoFiles []string
	Files       []string
}

/*
Load reads every package of the module, finding its root by walking up
from the working directory to the go.mod that declares it.
*/
func Load(tb testing.TB, module string) *Module {
	tb.Helper()

	root, err := findRoot(module)
	if err != nil {
		tb.Fatalf("load module %s: %v", module, err)
	}

	return LoadDir(tb, root)
}

/*
LoadDir reads every package of the module whose go.mod is in the root
directory. Directories the go command ignores are skipped: testdata,
vendor, nested modules and names starting with a dot or underscore.
*/
func LoadDir(tb testing.TB, root string) *Module {
	tb.Helper()

	module, ok := moduleOf(root)
	if !ok {
		tb.Fatalf("load module: no go.mod declares a module in %s", root)
	}

	m := &Module{tb: tb, path: module, root: root}

	if err := filepath.WalkDir(root, m.visit); err != nil {
		tb.Fatalf("load module %s: %v", module, err)
	}

	return m
}

/*
Packages returns the loaded packages.
*/
func (m *Module) Packages() []*Package {
	return m.packages
}

/*
visit loads the package in a directory of the module.
*/
func (m *Module) visit(dir string, entry fs.DirEntry, err error) error {
	if err != nil || !entry.IsDir() {
		return err
	}

	if dir != m.root && skipped(dir) {
		return filepath.SkipDir
	}

	pkg, err := build.ImportDir(dir, build.ImportComment)
	if _, ok := errors.AsType[*build.NoGoError](err); ok {
		return nil
	}

	if err != nil {
		return fmt.Errorf("import %s: %w", dir, err)
	}

	rel, err := filepath.Rel(m.root, dir)
	if err != nil {
		return fmt.Errorf("locate %s: %w", dir, err)
	}

	files, err := m.files(dir)
	if err != nil {
		return err
	}

	m.packages = append(m.packages, &Package{
		Path:        filepath.ToSlash(rel),
		Dir:         dir,
		Imports:     pkg.Imports,
		TestImports: append(append([]string(nil), pkg.TestImports...), pkg.XTestImports...),
		GoFiles:     pkg.GoFiles,
		TestGoFiles: append(append([]string(nil), pkg.TestGoFiles...), pkg.XTestGoFiles...),
		Files:       files,
	})

	return nil
}

/*
files lists the files of the package directory and of its testdata,
relative to the module root.
*/
func (m *Module) files(dir string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, m.relative(filepath.Join(dir, entry.Name())))
		}
	}

	testdata := filepath.Join(dir, "testdata")
	if !exists(testdata) {
		return files, nil
	}

	err = filepath.WalkDir(testdata, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			files = append(files, m.relative(path))
		}

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", testdata, err)
	}

	return files, nil
}

func (m *Module) relative(path string) string {
	rel, err := filepath.Rel(m.root, path)
	if err != nil {
		return path
	}

	return filepath.ToSlash(rel)
}

func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

/*
skipped reports whether the go command ignores the directory: testdata,
vendor, hidden and underscore directories, and nested modules.
*/
func skipped(dir string) bool {
	name := filepath.Base(dir)
	if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}

	return exists(filepath.Join(dir, "go.mod"))
}

/*
findRoot walks up from the working directory to the directory whose
go.mod declares the module.
*/
func findRoot(module string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("find working directory: %w", err)
	}

	for {
		if declared, ok := moduleOf(dir); ok && declared == module {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod declares %s above the working directory", module)
		}

		dir = parent
	}
}

/*
moduleOf returns the module the go.mod file in the directory declares.
*/
func moduleOf(dir string) (string, bool) {
	gomod, err := fs.ReadFile(os.DirFS(dir), "go.mod")
	if err != nil {
		return "", false
	}

	for line := range strings.Lines(string(gomod)) {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), true
		}
	}

	return "", false
}
