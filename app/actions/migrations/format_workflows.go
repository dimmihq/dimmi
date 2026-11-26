/*
|--------------------------------------------------------------------------
| Workflow Records
|--------------------------------------------------------------------------
|
| An automation of the other tool, kept as it was: its name, what starts
| it, its steps with whether each has a dimmi counterpart, and the raw
| definition. dimmi does not rebuild workflows yet; it keeps them in the
| ledger so a later version can, and so the migration's report lists
| what a team will need to set up again.
|
*/

package migrations

import (
	"context"
	"encoding/json"
)

/*
Workflow is one automation of the other tool, never turned on here.
*/
type Workflow struct {
	Base
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Trigger     string          `json:"trigger,omitempty"`
	Channels    []string        `json:"channels,omitempty"`
	Enabled     bool            `json:"enabled,omitempty"`
	Steps       []WorkflowStep  `json:"steps,omitempty"`
	Unsupported []string        `json:"unsupported,omitempty"`
	Definition  json.RawMessage `json:"definition,omitempty"`
}

/*
WorkflowStep is one step of a workflow: its type in the other tool, the
dimmi step it maps to, and whether dimmi has one.
*/
type WorkflowStep struct {
	SourceType string `json:"source_type"`
	Type       string `json:"type,omitempty"`
	Supported  bool   `json:"supported,omitempty"`
}

func (w *Workflow) validate() error { return required("name", w.Name) }

func (w *Workflow) refs() []Ref { return nil }

/*
apply keeps the workflow in the ledger and reports it unsupported, since
dimmi does not rebuild workflows yet.
*/
func (w *Workflow) apply(_ context.Context, _ *step) (outcome, error) {
	return outcome{label: w.Name}, ErrNotSupportedYet
}
