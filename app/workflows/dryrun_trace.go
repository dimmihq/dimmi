/*
|--------------------------------------------------------------------------
| Dry Run Traces
|--------------------------------------------------------------------------
|
| What a dry run reports: whether the trigger would start the workflow,
| every step and the path it took, the variables set, the actions that
| would run, and how it ended.
|
*/

package workflows

/*
DryTrace is what a dry run did: whether the workflow would start, every
step and the path it took, the variables set, the actions that would
run, and how it ended.
*/
type DryTrace struct {
	Verdict DryVerdict     `json:"verdict"`
	Steps   []DryStep      `json:"steps"`
	Vars    map[string]any `json:"vars"`
	Actions []DryAction    `json:"actions"`
	End     string         `json:"end"`
}

/*
DryVerdict says whether the trigger would start the workflow, and the
check that refused it when not.
*/
type DryVerdict struct {
	Started    bool   `json:"started"`
	Reason     string `json:"reason,omitempty"`
	FailedRule string `json:"failed_rule,omitempty"`
}

/*
DryStep is one step a dry run took: which, how it went, the path it
left by with the path's label, and what it noted.
*/
type DryStep struct {
	StepID    string         `json:"step_id"`
	Kind      string         `json:"kind"`
	Outcome   string         `json:"outcome"`
	PathID    string         `json:"path_id,omitempty"`
	PathLabel string         `json:"path_label,omitempty"`
	Skipped   bool           `json:"skipped,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
}

/*
DryAction is an action the workflow would take: the step, its kind,
what it would do in words, and whether a mock answered for it.
*/
type DryAction struct {
	StepID  string         `json:"step_id"`
	Kind    string         `json:"kind"`
	Summary string         `json:"summary"`
	Detail  map[string]any `json:"detail,omitempty"`
	Mocked  bool           `json:"mocked,omitempty"`
}
