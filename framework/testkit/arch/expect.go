/*
|--------------------------------------------------------------------------
| Expectations
|--------------------------------------------------------------------------
|
| An expectation picks packages and states a rule about them. Every
| broken rule is reported with the package and what broke it, and the
| test goes on, so one run lists every violation instead of the first.
|
| Import rules cover test files too, unless the expectation says it is
| only about the code that ships, because a test that reaches across a
| boundary couples the two sides just as much.
|
*/

package arch

import (
	"slices"
	"strings"
)

/*
Expectation is a set of packages rules are stated about.
*/
type Expectation struct {
	module      *Module
	patterns    []string
	except      []string
	withoutTest bool
}

/*
Expect returns an expectation about the packages matching the patterns.
A pattern that matches no package fails the test, so a rule cannot
silently check nothing.
*/
func (m *Module) Expect(patterns ...string) *Expectation {
	m.tb.Helper()

	e := &Expectation{module: m, patterns: patterns}

	for _, pattern := range patterns {
		if !slices.ContainsFunc(m.packages, func(p *Package) bool { return matches(pattern, p.Path) }) {
			m.tb.Errorf("arch: no package matches %q", pattern)
		}
	}

	return e
}

/*
Except returns the expectation without the packages matching the
patterns.
*/
func (e *Expectation) Except(patterns ...string) *Expectation {
	clone := *e
	clone.except = append(slices.Clone(e.except), patterns...)

	return &clone
}

/*
IgnoringTests returns the expectation about the code that ships only,
leaving test files out of import rules.
*/
func (e *Expectation) IgnoringTests() *Expectation {
	clone := *e
	clone.withoutTest = true

	return &clone
}

/*
NotToImport checks that no package imports a package matching the
patterns.
*/
func (e *Expectation) NotToImport(patterns ...string) *Expectation {
	e.module.tb.Helper()

	e.eachImport(func(p *Package, imported string) {
		if pattern, ok := e.matchAny(patterns, imported); ok {
			e.module.tb.Errorf("arch: %s imports %s, which matches %q", p.Path, imported, pattern)
		}
	})

	return e
}

/*
ToOnlyImport checks that the packages import nothing but packages
matching the patterns.
*/
func (e *Expectation) ToOnlyImport(patterns ...string) *Expectation {
	e.module.tb.Helper()

	e.eachImport(func(p *Package, imported string) {
		if _, ok := e.matchAny(patterns, imported); !ok {
			e.module.tb.Errorf("arch: %s imports %s, which is not one of %s",
				p.Path, imported, strings.Join(patterns, ", "))
		}
	})

	return e
}

/*
selected returns the packages the expectation is about.
*/
func (e *Expectation) selected() []*Package {
	var selected []*Package

	for _, p := range e.module.packages {
		in := slices.ContainsFunc(e.patterns, func(pattern string) bool { return matches(pattern, p.Path) })
		out := slices.ContainsFunc(e.except, func(pattern string) bool { return matches(pattern, p.Path) })

		if in && !out {
			selected = append(selected, p)
		}
	}

	return selected
}

/*
eachImport calls the function for every distinct import of every
selected package.
*/
func (e *Expectation) eachImport(fn func(p *Package, imported string)) {
	for _, p := range e.selected() {
		imports := slices.Clone(p.Imports)
		if !e.withoutTest {
			imports = append(imports, p.TestImports...)
		}

		slices.Sort(imports)

		for _, imported := range slices.Compact(imports) {
			if imported != e.module.importPath(p) {
				fn(p, imported)
			}
		}
	}
}

/*
importPath returns the package's full import path.
*/
func (m *Module) importPath(p *Package) string {
	if p.Path == "." {
		return m.path
	}

	return m.path + "/" + p.Path
}

func (e *Expectation) matchAny(patterns []string, imported string) (string, bool) {
	for _, pattern := range patterns {
		if matchesImport(pattern, e.module.path, imported) {
			return pattern, true
		}
	}

	return "", false
}
