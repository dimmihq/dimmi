/*
|--------------------------------------------------------------------------
| Example Sources
|--------------------------------------------------------------------------
|
| The example file is its own source for the values it sets, and the
| structs' comments are the source for what it says of each section.
| Both are read from disk when the example is written, which happens in
| development and in tests, never in a running application.
|
*/

package config

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
)

/*
currentExample reads the values the example at the path sets. A path
that is empty or names no file sets none.
*/
func currentExample(path string) (map[string]any, error) {
	if path == "" {
		return map[string]any{}, nil
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read the current example: %w", err)
	}

	parsed, err := yaml.Parser().Unmarshal(raw)
	if err != nil {
		return nil, fmt.Errorf("parse the current example: %w", err)
	}

	return parsed, nil
}

/*
exampleDocs reads what each struct's comment says, a package at a time.
*/
type exampleDocs struct {
	module string
	root   string
	read   map[string]map[string]string
}

/*
of returns the comment above the struct type, or "" for a type outside
the module.
*/
func (d exampleDocs) of(t reflect.Type) string {
	dir, ok := strings.CutPrefix(t.PkgPath(), strings.TrimSuffix(d.module, "/")+"/")
	if !ok || d.module == "" || t.Name() == "" {
		return ""
	}

	dir = strings.TrimSuffix(dir, "_test")

	if _, read := d.read[dir]; !read {
		d.read[dir] = packageDocs(filepath.Join(d.root, dir))
	}

	return d.read[dir][t.Name()]
}

/*
packageDocs maps each type declared in the directory to its comment.
*/
func packageDocs(dir string) map[string]string {
	out := map[string]string{}

	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, path := range files {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		for _, decl := range file.Decls {
			name, doc := typeDoc(decl)
			if name != "" {
				out[name] = doc
			}
		}
	}

	return out
}

func typeDoc(decl ast.Decl) (name, doc string) {
	typed, ok := decl.(*ast.GenDecl)
	if !ok || typed.Tok != token.TYPE || typed.Doc == nil || len(typed.Specs) != 1 {
		return "", ""
	}

	spec, ok := typed.Specs[0].(*ast.TypeSpec)
	if !ok {
		return "", ""
	}

	return spec.Name.Name, strings.TrimSpace(typed.Doc.Text())
}
