/*
|--------------------------------------------------------------------------
| Checking the Step Tree
|--------------------------------------------------------------------------
|
| Every step must be a known kind the trigger allows, with an id unique
| in the whole definition, and with exactly the paths its kind may
| have. An opener runs in the widget before a conversation exists, so
| it may only show messages and then one question with buttons.
|
*/

package workflowspec

import (
	"regexp"
	"slices"
)

/*
idPattern is the shape of step and path ids.
*/
func idPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[sp]_[a-z0-9_]{6,16}$`)
}

/*
checkSteps checks a list of steps at a nesting depth.
*/
func (c *checker) checkSteps(steps []Step, depth int) {
	if depth > MaxPathDepth {
		c.fail(Issue{Code: CodeLimitExceeded, Message: "Nest paths 12 levels deep at most."})
		return
	}

	for _, s := range steps {
		c.steps++
		c.checkStep(s, depth)
	}
}

/*
checkStep checks one step, its settings and its paths.
*/
func (c *checker) checkStep(s Step, depth int) {
	c.checkID(s.ID, "s_", s.ID)

	k, ok := FindKind(s.Kind)
	if !ok {
		c.fail(Issue{Code: CodeSchema, StepID: s.ID, Message: "No step kind has this name."})
		return
	}

	if !c.trigger.AllowsStep(s.Kind) {
		c.fail(Issue{
			Code: CodeStepUnavailableForTrigger, StepID: s.ID,
			Message: "This trigger cannot use this step.",
		})
	}

	if !k.Ready {
		c.fail(Issue{Code: CodeStepNotAvailable, StepID: s.ID, Message: "This step cannot run yet."})
	}

	c.checkPaths(s, k)
	c.checkSettings(s)

	for _, p := range s.Paths {
		c.checkSteps(p.Steps, depth+1)
	}
}

/*
checkID refuses a malformed or repeated id.
*/
func (c *checker) checkID(id, prefix, stepID string) {
	if !idPattern().MatchString(id) || id[:2] != prefix {
		c.fail(Issue{Code: CodeSchema, StepID: stepID, Message: "Ids look like " + prefix + "abc123."})
		return
	}

	if c.ids[id] {
		c.fail(Issue{Code: CodeDuplicateID, StepID: stepID, Message: "Another step or path has this id."})
	}

	c.ids[id] = true
}

/*
checkPaths checks a step's paths against its kind: fixed outcomes the
kind knows, at most one of each, required ones present, and the right
number of user paths.
*/
func (c *checker) checkPaths(s Step, k Kind) {
	seen := map[string]bool{}
	users := 0

	for _, p := range s.Paths {
		c.checkID(p.ID, "p_", s.ID)

		if p.Outcome == "" {
			users++
			continue
		}

		if !slices.Contains(k.Fixed, p.Outcome) || seen[p.Outcome] {
			c.fail(Issue{Code: CodeSchema, StepID: s.ID, PathID: p.ID, Message: "This step has no such path."})
		}

		seen[p.Outcome] = true
	}

	for _, outcome := range k.Required {
		if !seen[outcome] {
			c.fail(Issue{Code: CodeMissingSetting, StepID: s.ID, Message: "This step needs its " + outcome + " path."})
		}
	}

	if users < k.MinPaths || users > k.MaxPaths {
		c.fail(Issue{Code: CodeLimitExceeded, StepID: s.ID, Message: "This step has too few or too many paths."})
	}
}

/*
checkOpener enforces the opener's shape: up to five messages, then one
buttons step. Nothing else can be shown before a conversation exists.
*/
func (c *checker) checkOpener() {
	steps := c.spec.Steps
	i := 0

	for i < len(steps) && steps[i].Kind == KindMessage {
		i++
	}

	if i > maxOpenerHead || i != len(steps)-1 || steps[i].Kind != KindButtons {
		c.fail(Issue{
			Code: CodeOpenerShape, Field: "steps",
			Message: "An opener shows messages, then one question with buttons, and nothing else at the top.",
		})
	}
}
