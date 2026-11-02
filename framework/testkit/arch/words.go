/*
|--------------------------------------------------------------------------
| Banned Words
|--------------------------------------------------------------------------
|
| Some words must not appear in some packages at all, such as product
| words in code meant to be reused by any product. A word counts wherever
| a reader meets it: in identifiers, string literals, comments and file
| names, in test files too. Identifiers are split at case changes,
| underscores and dashes, so "LineItem" and "line_item" both contain
| the stem "lineitem", and a stem matches the start of a word, so
| "orders" contains "order" while "borders" does not.
|
| The module's own import path is not a word: it may name the product,
| and every package must be able to quote it.
|
*/

package arch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

var majorVersion = regexp.MustCompile(`^v\d+$`)

/*
finding is one banned word found in a package.
*/
type finding struct {
	position string
	where    string
	word     string
	stem     string
}

/*
NotToContainWords checks that no identifier, string literal, comment or
file name of the packages contains a word starting with one of the
stems, compared without case, dashes or underscores.
*/
func (e *Expectation) NotToContainWords(stems ...string) *Expectation {
	e.module.tb.Helper()

	s := scanner{
		stems:      normalizeStems(stems),
		modulePath: regexp.MustCompile(regexp.QuoteMeta(e.module.path) + `[A-Za-z0-9_./-]*`),
		fset:       token.NewFileSet(),
	}

	for _, p := range e.selected() {
		for _, f := range s.scanPackage(e.module, p, e.files(p)) {
			e.module.tb.Errorf("arch: %s: %s %q contains %q", f.position, f.where, f.word, f.stem)
		}
	}

	return e
}

/*
scanner looks for the stems in source files.
*/
type scanner struct {
	stems      []string
	modulePath *regexp.Regexp
	fset       *token.FileSet
}

/*
scanPackage looks for the stems in the names of the package's files and
in its Go files.
*/
func (s scanner) scanPackage(m *Module, p *Package, files packageFiles) []finding {
	var findings []finding

	for _, name := range files.names {
		for word, stem := range s.find(name) {
			findings = append(findings, finding{position: name, where: "file name", word: word, stem: stem})
		}
	}

	for _, name := range files.code {
		findings = append(findings, s.scanFile(m, filepath.Join(p.Dir, name))...)
	}

	return findings
}

/*
packageFiles are the files of a package an expectation reads: the names
of every file, relative to the module root, and the Go files by name.
*/
type packageFiles struct {
	names []string
	code  []string
}

/*
files returns the package's files, without its test files when the
expectation leaves tests out.
*/
func (e *Expectation) files(p *Package) packageFiles {
	if !e.withoutTest {
		return packageFiles{names: p.Files, code: append(slices.Clone(p.GoFiles), p.TestGoFiles...)}
	}

	names := slices.DeleteFunc(slices.Clone(p.Files), func(name string) bool {
		return strings.HasSuffix(name, "_test.go")
	})

	return packageFiles{names: names, code: p.GoFiles}
}

/*
scanFile looks for the stems in a Go file's identifiers, string literals
other than import paths, and comments.
*/
func (s scanner) scanFile(m *Module, path string) []finding {
	file, err := parser.ParseFile(s.fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		m.tb.Errorf("arch: parse %s: %v", m.relative(path), err)

		return nil
	}

	f := &fileScan{scanner: s, module: m, imports: importNames(file)}

	ast.Inspect(file, f.visit)

	for _, group := range file.Comments {
		for _, comment := range group.List {
			f.report(comment.Pos(), "comment", comment.Text)
		}
	}

	return f.findings
}

/*
fileScan collects the findings of one file. Imports holds the names the
file's imported packages are known by.
*/
type fileScan struct {
	scanner
	module   *Module
	imports  map[string]bool
	findings []finding
}

/*
report records every stem in the text, at the line of the text it is
on.
*/
func (f *fileScan) report(pos token.Pos, where, text string) {
	start := f.fset.Position(pos)

	for i, line := range strings.Split(text, "\n") {
		for word, stem := range f.find(line) {
			f.findings = append(f.findings, finding{
				position: fmt.Sprintf("%s:%d", f.module.relative(start.Filename), start.Line+i),
				where:    where, word: word, stem: stem,
			})
		}
	}
}

/*
visit reports the identifier or string literal at the node. Comments are
reported once from the file's list instead, and import paths not at all.
Names a file uses from another package, such as a library's type, are
not its own words: a framework package's names are reported where they
are declared.
*/
func (f *fileScan) visit(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.CommentGroup:
		return false
	case *ast.ImportSpec:
		if n.Name != nil {
			f.report(n.Name.Pos(), "identifier", n.Name.Name)
		}

		return false
	case *ast.SelectorExpr:
		if x, ok := n.X.(*ast.Ident); ok && f.imports[x.Name] {
			f.report(x.Pos(), "identifier", x.Name)

			return false
		}
	case *ast.Ident:
		f.report(n.Pos(), "identifier", n.Name)
	case *ast.BasicLit:
		if n.Kind == token.STRING {
			f.report(n.Pos(), "string", literal(n.Value))
		}
	}

	return true
}

/*
importNames returns the names the file's imports are known by: the
alias when there is one, otherwise the last element of the path, or the
one before it when the last is a major version such as v5. Prefixes like
"go-" and suffixes like ".v3" are dropped too, as package names drop
them.
*/
func importNames(file *ast.File) map[string]bool {
	names := map[string]bool{}

	for _, spec := range file.Imports {
		if spec.Name != nil {
			names[spec.Name.Name] = true

			continue
		}

		elements := strings.Split(literal(spec.Path.Value), "/")
		last := elements[len(elements)-1]

		if len(elements) > 1 && majorVersion.MatchString(last) {
			last = elements[len(elements)-2]
		}

		last, _, _ = strings.Cut(last, ".")
		names[last] = true
		names[strings.ReplaceAll(strings.TrimPrefix(last, "go-"), "-", "")] = true
	}

	return names
}

/*
find returns each word of the text that starts with a stem, mapped to
the stem, ignoring the module's import path. Words are joined with the
ones after them, so a stem split by a case change still matches.
*/
func (s scanner) find(text string) map[string]string {
	found := map[string]string{}

	for _, tok := range tokens(s.modulePath.ReplaceAllString(text, " ")) {
		parts := split(tok)
		for i := range parts {
			if stem, ok := s.matchStem(strings.Join(parts[i:], "")); ok {
				found[tok] = stem
			}
		}
	}

	return found
}

func (s scanner) matchStem(joined string) (string, bool) {
	for _, stem := range s.stems {
		if strings.HasPrefix(joined, stem) {
			return stem, true
		}
	}

	return "", false
}

/*
tokens splits text into runs of letters, digits, dashes and
underscores.
*/
func tokens(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-'
	})
}

/*
split breaks a token into lower-case words at dashes, underscores and
case changes: "HTTPServer_line-Item" gives "http", "server", "line" and
"item".
*/
func split(tok string) []string {
	var parts []string

	for _, piece := range strings.FieldsFunc(tok, func(r rune) bool { return r == '_' || r == '-' }) {
		runes := []rune(piece)
		start := 0

		for i := 1; i < len(runes); i++ {
			if boundary(runes, i) {
				parts = append(parts, strings.ToLower(string(runes[start:i])))
				start = i
			}
		}

		parts = append(parts, strings.ToLower(string(runes[start:])))
	}

	return parts
}

/*
boundary reports whether a new word starts at the rune: a capital after
a small letter, or the last capital of a run followed by a small letter.
*/
func boundary(runes []rune, i int) bool {
	if !unicode.IsUpper(runes[i]) {
		return false
	}

	if unicode.IsLower(runes[i-1]) {
		return true
	}

	return unicode.IsUpper(runes[i-1]) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
}

/*
normalizeStems lowers the stems and drops their dashes and underscores,
so "line_item" is matched as "lineitem".
*/
func normalizeStems(stems []string) []string {
	normalized := make([]string, 0, len(stems))
	for _, stem := range stems {
		normalized = append(normalized, strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(stem)))
	}

	return normalized
}

/*
literal returns the text of a string literal, or the literal as written
when it cannot be unquoted.
*/
func literal(value string) string {
	text, err := strconv.Unquote(value)
	if err != nil {
		return value
	}

	return text
}
