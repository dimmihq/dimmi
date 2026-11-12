/*
|--------------------------------------------------------------------------
| Workflow Run Resource
|--------------------------------------------------------------------------
|
| A run as the builder shows it: how it ended, what started it, the
| conversation it worked and the run that called it, with every step
| it executed, the path it took and the facts the step kept.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkflowRun is one run with its trace.
*/
type WorkflowRun struct {
	ID             string            `json:"id"`
	Object         string            `json:"object"`
	Status         string            `json:"status"`
	EndReason      *string           `json:"end_reason"`
	Trigger        string            `json:"trigger"`
	ConversationID *string           `json:"conversation_id"`
	ParentRunID    *string           `json:"parent_run_id"`
	StartedAt      time.Time         `json:"started_at"`
	EndedAt        *time.Time        `json:"ended_at"`
	Steps          []WorkflowRunStep `json:"steps"`
}

/*
WorkflowRunStep is one step of a run's trace.
*/
type WorkflowRunStep struct {
	StepID     string          `json:"step_id"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	Outcome    *string         `json:"outcome"`
	PathID     *string         `json:"path_id"`
	Detail     json.RawMessage `json:"detail"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at"`
}

/*
NewWorkflowRuns shapes runs with their traces.
*/
func NewWorkflowRuns(traces []workflows.RunTrace) []WorkflowRun {
	out := make([]WorkflowRun, 0, len(traces))
	for _, t := range traces {
		r := t.Run
		run := WorkflowRun{
			ID: models.PublicID(models.PrefixWorkflowRun, r.ID), Object: "workflow_run", Status: r.Status,
			EndReason: r.EndReason, Trigger: r.Trigger, StartedAt: r.StartedAt, EndedAt: r.EndedAt,
			ConversationID: publicOrNil(models.PrefixConversation, r.ConversationID),
			ParentRunID:    publicOrNil(models.PrefixWorkflowRun, r.ParentRunID), Steps: []WorkflowRunStep{},
		}

		for _, s := range t.Steps {
			run.Steps = append(run.Steps, WorkflowRunStep{
				StepID: s.StepID, Kind: s.Kind, Status: s.Status, Outcome: s.Outcome, PathID: s.PathID,
				Detail: s.Detail, StartedAt: s.StartedAt, FinishedAt: s.FinishedAt,
			})
		}

		out = append(out, run)
	}

	return out
}

/*
publicOrNil returns the public id, nil for none.
*/
func publicOrNil(prefix models.Prefix, id *ids.UUID) *string {
	if id == nil {
		return nil
	}

	public := models.PublicID(prefix, *id)

	return &public
}
