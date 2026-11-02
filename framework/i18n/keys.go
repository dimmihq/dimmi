/*
|--------------------------------------------------------------------------
| Message Key Check
|--------------------------------------------------------------------------
|
| Every shipped language must be complete, and every message must still
| be in use, or readers meet English in the middle of their language and
| translators keep paying for text nobody sees. The check compares the
| keys each locale's files hold with each other and with the keys the
| code asks for, found by reading the Go source for calls to T and
| Lookup with a literal key.
|
|   T(ctx, "auth.failed", nil)         uses "auth.failed"
|   a.Lang.Lookup(ctx, "auth.failed", nil) uses "auth.failed"
|
| A key built at run time cannot be seen in the source, so the caller
| lists such keys among the used ones itself, as it does for the
| framework's own messages.
|
*/

package i18n

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"
)

/*
Report lists the problems of a set of message files. Missing maps each
locale to the keys another locale has and it lacks. Unused lists the
keys no code uses.
*/
type Report struct {
	Missing map[string][]string
	Unused  []string
}

/*
Clean reports whether the files have no problem.
*/
func (r Report) Clean() bool {
	return len(r.Missing) == 0 && len(r.Unused) == 0
}

/*
Keys returns the keys of every locale in the files, sorted.
*/
func Keys(files ...fs.FS) (map[string][]string, error) {
	sets := map[string]map[string]bool{}

	for _, dir := range files {
		parsed, err := readAll(dir)
		if err != nil {
			return nil, err
		}

		for _, file := range parsed {
			locale := file.tag.String()
			if sets[locale] == nil {
				sets[locale] = map[string]bool{}
			}

			for key := range file.messages {
				sets[locale][key] = true
			}
		}
	}

	keys := make(map[string][]string, len(sets))
	for locale, set := range sets {
		keys[locale] = slices.Sorted(maps.Keys(set))
	}

	return keys, nil
}

/*
Check reports the keys a locale of the files lacks while another has
them, and the keys missing from used.
*/
func Check(used []string, files ...fs.FS) (Report, error) {
	keys, err := Keys(files...)
	if err != nil {
		return Report{}, err
	}

	all := map[string]bool{}
	for _, list := range keys {
		for _, key := range list {
			all[key] = true
		}
	}

	report := Report{Missing: map[string][]string{}}

	for locale, list := range keys {
		if missing := absent(all, list); len(missing) > 0 {
			report.Missing[locale] = missing
		}
	}

	report.Unused = absent(all, used)

	return report, nil
}

/*
absent returns the keys of the set that the list lacks, sorted.
*/
func absent(set map[string]bool, list []string) []string {
	var missing []string

	for _, key := range slices.Sorted(maps.Keys(set)) {
		if !slices.Contains(list, key) {
			missing = append(missing, key)
		}
	}

	return missing
}

/*
ScanKeys returns the keys the Go code under the directory passes as a
literal to a function or method named T or Lookup, sorted. Tests,
vendored code, test data and hidden directories are skipped.
*/
func ScanKeys(code fs.FS) ([]string, error) {
	found := map[string]bool{}
	fset := token.NewFileSet()

	err := fs.WalkDir(code, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return skipDir(name)
		}

		if path.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		return scanFile(code, fset, name, found)
	})
	if err != nil {
		return nil, fmt.Errorf("scan message keys: %w", err)
	}

	return slices.Sorted(maps.Keys(found)), nil
}

/*
skipDir skips the directories that hold no application code.
*/
func skipDir(name string) error {
	base := path.Base(name)
	ignored := slices.Contains([]string{"vendor", "node_modules", "testdata"}, base)

	if name != "." && (ignored || strings.HasPrefix(base, ".")) {
		return fs.SkipDir
	}

	return nil
}

/*
scanFile adds the literal keys of one Go file to found.
*/
func scanFile(code fs.FS, fset *token.FileSet, name string, found map[string]bool) error {
	src, err := fs.ReadFile(code, name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}

	ast.Inspect(file, func(node ast.Node) bool {
		if key, ok := literalKey(node); ok {
			found[key] = true
		}

		return true
	})

	return nil
}

/*
literalKey returns the key of a call to T or Lookup whose second
argument is a string literal.
*/
func literalKey(node ast.Node) (string, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) < 2 || !slices.Contains([]string{"T", "Lookup"}, calleeName(call.Fun)) {
		return "", false
	}

	literal, ok := call.Args[1].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}

	key, err := strconv.Unquote(literal.Value)

	return key, err == nil
}

/*
calleeName returns the name of a called function or method.
*/
func calleeName(fun ast.Expr) string {
	switch callee := fun.(type) {
	case *ast.Ident:
		return callee.Name
	case *ast.SelectorExpr:
		return callee.Sel.Name
	default:
		return ""
	}
}
