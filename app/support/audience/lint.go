/*
|--------------------------------------------------------------------------
| Rule Warnings
|--------------------------------------------------------------------------
|
| Some rules are valid but almost never what their author meant: two
| "is not" conditions joined by "any" match nearly everyone, and two
| different "is" values joined by "all" match nobody. Page conditions
| are sent to browsers to be checked there, so their values are public.
| Each is a warning with the path of the group or leaf, never a refusal:
| someone may mean it.
|
*/

package audience

import (
	"fmt"
	"slices"
)

/*
The warning codes.
*/
const (
	WarnNegativeOr    = "negative_or"
	WarnImpossibleAnd = "impossible_and"
	WarnPageIsPublic  = "page_rule_is_public"
)

/*
Warning is a rule that is allowed but probably wrong.
*/
type Warning struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

/*
Lint returns the warnings of an audience.
*/
func Lint(a Audience) []Warning {
	if a.Rules == nil {
		return []Warning{}
	}

	out := append([]Warning{}, lintNode(*a.Rules, "rules")...)
	page := false
	walk(a.Rules, func(leaf Rule) { page = page || isPage(leaf.Field) })

	if page {
		out = append(out, Warning{
			Code: WarnPageIsPublic, Path: "rules",
			Message: "Page conditions are checked in the browser, so anyone can read their values.",
		})
	}

	return out
}

/*
lintNode checks one group and its children.
*/
func lintNode(r Rule, path string) []Warning {
	if r.IsLeaf() {
		return nil
	}

	var out []Warning
	if r.Not != nil {
		return lintNode(*r.Not, path+".not")
	}

	if w, ok := negativeOr(r.Any, path+".any"); ok {
		out = append(out, w)
	}

	if w, ok := impossibleAnd(r.All, path+".all"); ok {
		out = append(out, w)
	}

	for i, child := range r.All {
		out = append(out, lintNode(child, fmt.Sprintf("%s.all[%d]", path, i))...)
	}

	for i, child := range r.Any {
		out = append(out, lintNode(child, fmt.Sprintf("%s.any[%d]", path, i))...)
	}

	return out
}

/*
negativeOr finds two negative conditions on one field joined by any.
*/
func negativeOr(children []Rule, path string) (Warning, bool) {
	seen := map[string]bool{}
	for _, c := range children {
		if !c.IsLeaf() || !negative(c.Op) || c.Op == OpNotExists {
			continue
		}

		if seen[c.Field] {
			return Warning{
				Code: WarnNegativeOr, Path: path,
				Message: "Two \"is not\" conditions on one field joined by \"any\" match nearly everyone. " +
					"Use \"all\" to leave out both values.",
			}, true
		}

		seen[c.Field] = true
	}

	return Warning{}, false
}

/*
impossibleAnd finds two different equal-to values on one field joined
by all.
*/
func impossibleAnd(children []Rule, path string) (Warning, bool) {
	values := map[string][]string{}
	for _, c := range children {
		if !c.IsLeaf() || c.Op != OpEq {
			continue
		}

		v := string(c.Value)
		if seen := values[c.Field]; len(seen) > 0 && !slices.Contains(seen, v) {
			return Warning{
				Code: WarnImpossibleAnd, Path: path,
				Message: "One field cannot equal two different values at once, so this matches nobody. " +
					"Use \"any\" to match either.",
			}, true
		}

		values[c.Field] = append(values[c.Field], v)
	}

	return Warning{}, false
}

/*
isPage reports whether a field is read in the browser.
*/
func isPage(field string) bool {
	f, ok := Base().Lookup(field)
	return ok && f.Where == Client
}
