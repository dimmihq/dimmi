/*
|--------------------------------------------------------------------------
| Package Patterns
|--------------------------------------------------------------------------
|
| Rules name packages the way people do: "framework/..." for a tree,
| "framework/.../drivertest" for every contract suite, "app/actions" for
| one package. Patterns without a domain in their first element are
| relative to the module; others are full import paths.
|
*/

package arch

import (
	"path"
	"strings"
)

/*
Std stands for every package of the standard library in an import rule.
*/
const Std = "std"

/*
matches reports whether the slash-separated path matches the pattern.
*/
func matches(pattern, target string) bool {
	return matchElements(strings.Split(pattern, "/"), strings.Split(target, "/"))
}

func matchElements(pattern, target []string) bool {
	if len(pattern) == 0 {
		return len(target) == 0
	}

	if pattern[0] == "..." {
		for i := range len(target) + 1 {
			if matchElements(pattern[1:], target[i:]) {
				return true
			}
		}

		return false
	}

	if len(target) == 0 {
		return false
	}

	ok, err := path.Match(pattern[0], target[0])
	if err != nil || !ok {
		return false
	}

	return matchElements(pattern[1:], target[1:])
}

/*
matchesImport reports whether an import path matches the pattern, which
is Std, a full import path pattern or one relative to the module.
*/
func matchesImport(pattern, module, importPath string) bool {
	rel, inside := strings.CutPrefix(importPath, module+"/")
	if importPath == module {
		rel, inside = ".", true
	}

	if pattern == Std {
		return !inside && isStd(importPath)
	}

	if external(pattern) {
		return matches(pattern, importPath)
	}

	return inside && matches(pattern, rel)
}

/*
isStd reports whether the import path is in the standard library, whose
paths have no domain in their first element.
*/
func isStd(importPath string) bool {
	return !external(importPath) && importPath != "C"
}

/*
external reports whether the path starts with a domain, as paths
outside the standard library and the module do.
*/
func external(p string) bool {
	first, _, _ := strings.Cut(p, "/")

	return strings.Contains(first, ".")
}
