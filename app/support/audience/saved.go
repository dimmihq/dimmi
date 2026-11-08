/*
|--------------------------------------------------------------------------
| Saved Audience References
|--------------------------------------------------------------------------
|
| A rule names saved audiences by public id in "audience" conditions.
| Whatever stores a rule records those ids, so a saved audience in use
| is never deleted from under it and nesting never loops.
|
*/

package audience

import (
	"encoding/json"
	"slices"
)

/*
SavedIDs returns the saved audiences a rule names, each once, in the
order they first appear.
*/
func SavedIDs(r *Rule) []string {
	var out []string
	walk(r, func(leaf Rule) {
		if leaf.Field != "audience" {
			return
		}

		var listed []string
		if json.Unmarshal(leaf.Value, &listed) != nil {
			return
		}

		for _, id := range listed {
			if id != "" && !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	})

	return out
}

/*
walk calls fn for every leaf of a rule, in reading order.
*/
func walk(r *Rule, fn func(Rule)) {
	if r == nil {
		return
	}

	if r.IsLeaf() {
		fn(*r)
		return
	}

	walk(r.Not, fn)
	for i := range r.All {
		walk(&r.All[i], fn)
	}

	for i := range r.Any {
		walk(&r.Any[i], fn)
	}
}

/*
NeedsSaved reports whether the program names any saved audience, so a
caller loads saved audiences only for rules that use them.
*/
func (p *Program) NeedsSaved() bool {
	if p == nil {
		return false
	}

	return needsSaved(p.root)
}

/*
needsSaved walks compiled nodes for a saved audience condition.
*/
func needsSaved(n *node) bool {
	if n == nil {
		return false
	}

	if n.kind == "leaf" {
		return n.field.Type == TypeSaved
	}

	return slices.ContainsFunc(n.children, needsSaved)
}
