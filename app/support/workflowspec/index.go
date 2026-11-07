/*
|--------------------------------------------------------------------------
| Step Index
|--------------------------------------------------------------------------
|
| The engine moves a run's cursor by step id: to the next step on the
| same path, or into one of a step's paths. Paths never rejoin, so the
| end of a path is the end of the run. The index answers both moves
| without walking the tree on every step.
|
*/

package workflowspec

/*
Index locates every step of a definition.
*/
type Index struct {
	steps map[string]place
	first string
}

/*
place is where a step sits: its list and its position in it.
*/
type place struct {
	step  Step
	list  []Step
	index int
}

/*
NewIndex indexes a definition's steps.
*/
func NewIndex(s Spec) *Index {
	ix := &Index{steps: map[string]place{}}
	if len(s.Steps) > 0 {
		ix.first = s.Steps[0].ID
	}

	ix.add(s.Steps)

	return ix
}

/*
add indexes a list of steps and everything below them.
*/
func (ix *Index) add(list []Step) {
	for i, s := range list {
		ix.steps[s.ID] = place{step: s, list: list, index: i}
		for _, p := range s.Paths {
			ix.add(p.Steps)
		}
	}
}

/*
First returns the id of the first step, empty for an empty definition.
*/
func (ix *Index) First() string {
	return ix.first
}

/*
Step returns the step with the id.
*/
func (ix *Index) Step(id string) (Step, bool) {
	p, ok := ix.steps[id]
	return p.step, ok
}

/*
Next returns the id of the step after this one on its path, empty at
the end of the path.
*/
func (ix *Index) Next(id string) string {
	p, ok := ix.steps[id]
	if !ok || p.index+1 >= len(p.list) {
		return ""
	}

	return p.list[p.index+1].ID
}

/*
Enter returns the id of the first step of one of the step's paths,
empty when that path has no steps or does not exist.
*/
func (ix *Index) Enter(stepID, pathID string) string {
	s, ok := ix.Step(stepID)
	if !ok {
		return ""
	}

	for _, p := range s.Paths {
		if p.ID == pathID && len(p.Steps) > 0 {
			return p.Steps[0].ID
		}
	}

	return ""
}
