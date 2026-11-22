/*
|--------------------------------------------------------------------------
| Sub-Procedure Graph
|--------------------------------------------------------------------------
|
| A procedure may follow reusable sub-procedures, which may follow
| others. Publishing refuses a loop, where a procedure ends up following
| itself, and a chain deeper than MaxSubDepth, so expanding a procedure
| always ends.
|
*/

package procedure

/*
Cycle walks the sub-procedures root follows, reading each one's
references through refs, and returns the first loop found, as the
chain of public ids that closes it, or the chain that goes too deep.
Nil means neither.
*/
func Cycle(root string, refs func(id string) []string) []string {
	var found []string
	var visit func(id string, chain []string)
	visit = func(id string, chain []string) {
		if found != nil {
			return
		}
		for i, seen := range chain {
			if seen == id {
				found = append(append([]string{}, chain[i:]...), id)
				return
			}
		}
		chain = append(chain, id)
		if len(chain) > MaxSubDepth+1 {
			found = append([]string{}, chain...)
			return
		}
		for _, next := range refs(id) {
			visit(next, chain)
		}
	}
	visit(root, nil)

	return found
}
