/*
|--------------------------------------------------------------------------
| Workflow Simulation Checks
|--------------------------------------------------------------------------
|
| A workflow simulation takes every check an agent's does: words in a
| reply, an action called or not, a handover, and a check written as a
| sentence, which a model judges with its reason and may leave unsure.
| Three more read the workflow's own trace: whether the trigger started
| it, whether a step was reached, and which path a step took. An action
| a check names is a step's id or its kind, such as "tag", or one of
| the agent's own actions inside an agent step.
|
*/

package workflowsims

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/support/procedure"
	engine "github.com/dimmihq/dimmi/app/workflows"
)

/*
The checks only workflows have.
*/
const (
	AssertWorkflowStarted = "workflow_started"
	AssertStepReached     = "step_reached"
	AssertPathTaken       = "path_taken"
)

/*
own reports whether a check is one only workflows have.
*/
func own(a procedure.Assertion) bool {
	return slices.Contains([]string{AssertWorkflowStarted, AssertStepReached, AssertPathTaken}, a.Type)
}

/*
shared is the checks the agent's check reads, nil when there are none.
*/
func shared(checks []procedure.Assertion) []procedure.Assertion {
	var out []procedure.Assertion
	for _, a := range checks {
		if !own(a) {
			out = append(out, a)
		}
	}
	return out
}

/*
checkAssertions refuses a check only workflows have that names no step
or path.
*/
func checkAssertions(checks []procedure.Assertion) error {
	for _, a := range checks {
		bad := ""
		switch a.Type {
		case AssertStepReached:
			if a.Step == "" {
				bad = "Name the step."
			}
		case AssertPathTaken:
			if a.Step == "" || a.Branch == "" {
				bad = "Name the step and the path it takes."
			}
		}
		if bad != "" {
			return ErrSimulationInvalid.With("errors", []procedure.Issue{
				{Code: "invalid_setting", Field: "assertions", Message: bad},
			}).OnField("assertions")
		}
	}
	return nil
}

/*
marshal encodes checks for storage.
*/
func marshal(checks []procedure.Assertion) json.RawMessage {
	raw, _ := json.Marshal(checks)
	return raw
}

/*
judgeRun checks every check against the run: the workflow's own
against its trace, the rest as an agent's are.
*/
func (s *Simulations) judgeRun(ctx context.Context, ch *chat, trace *engine.DryTrace) []procedure.AssertionResult {
	checks := ch.script.assertions
	transcript := ch.transcript()
	judge := s.judge(ctx, ch.p.m, ch)
	out := make([]procedure.AssertionResult, 0, len(checks))
	for _, a := range checks {
		if own(a) {
			r := ownResult(a, trace)
			r.Type = a.Type
			out = append(out, r)
			continue
		}
		out = append(out, procedure.Assert([]procedure.Assertion{a}, ch.observed(trace), transcript, judge)...)
	}
	return out
}

/*
ownResult checks one of the workflow's own checks against its trace.
They name the workflow's own steps, so the steps of a workflow it calls
never count.
*/
func ownResult(a procedure.Assertion, trace *engine.DryTrace) procedure.AssertionResult {
	verdict := func(ok bool, yes, no string) procedure.AssertionResult {
		if ok {
			return procedure.AssertionResult{Passed: true, Message: yes}
		}
		return procedure.AssertionResult{Message: no}
	}
	switch a.Type {
	case AssertWorkflowStarted:
		want := a.Matched == nil || *a.Matched
		return verdict(trace.Verdict.Started == want, "the workflow started as expected",
			"the workflow did not start as expected")
	case AssertStepReached:
		reached := slices.ContainsFunc(trace.Steps, func(s engine.DryStep) bool {
			return s.Depth == 0 && s.StepID == a.Step
		})
		return verdict(reached, "step "+a.Step+" was reached", "the workflow never reached step "+a.Step+".")
	}
	for _, s := range trace.Steps {
		if s.Depth > 0 || s.StepID != a.Step {
			continue
		}
		label := cmp.Or(s.PathLabel, s.PathID, "its own path")
		return verdict(s.PathID == a.Branch, "step "+a.Step+" took "+label, "It took "+label+" instead.")
	}
	return procedure.AssertionResult{Message: "the workflow never reached step " + a.Step + "."}
}

/*
judge is the judge of a run's checks written as sentences, none when no
judge is set up. They are judged together, once, the first time one is
asked for.
*/
func (s *Simulations) judge(ctx context.Context, m authz.Membership, ch *chat) procedure.Judge {
	if s.Procedures == nil || s.Procedures.Judge == nil {
		return nil
	}
	transcript, wrote := ch.transcript(), ch.wrote
	var rubrics []string
	for _, a := range ch.script.assertions {
		if a.Type == procedure.AssertLLMRubric && !slices.Contains(rubrics, a.Rubric) {
			rubrics = append(rubrics, a.Rubric)
		}
	}
	var verdicts []procedure.Judgment
	var failed error
	asked := false
	return func(rubric, _ string) (procedure.Judgment, error) {
		if !asked {
			asked = true
			verdicts, failed = s.Procedures.Judge.JudgeRubrics(ctx, pipeline.Rubrics{
				WorkspaceID: m.Workspace.ID, MemberID: m.Member.ID, Rubrics: rubrics, Transcript: transcript,
				Wrote: wrote,
			})
		}
		i := slices.Index(rubrics, rubric)
		if failed != nil || i < 0 || i >= len(verdicts) {
			return procedure.Judgment{}, cmp.Or(failed, pipeline.ErrNoJudgment)
		}
		return verdicts[i], nil
	}
}

/*
unjudged is the error of a run with a check nobody could judge, empty
when every check was decided.
*/
func unjudged(results []procedure.AssertionResult) string {
	for _, r := range results {
		if r.Err != nil {
			return "The simulation failed: a check could not be judged (" + r.Err.Error() + "). Try again."
		}
	}
	return ""
}
