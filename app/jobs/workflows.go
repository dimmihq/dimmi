/*
|--------------------------------------------------------------------------
| Workflow Jobs
|--------------------------------------------------------------------------
|
| Workflows run off the requests that trigger them. One job matches an
| event against the live workflows, one advances the runs an event
| started, one after another in list order, and one moves a single run
| on: the rest of its steps, or its timer once due. Each job carries
| the run's sequence, so a job that arrives late finds nothing to do.
| One more resumes the runs a customer message answers.
|
*/

package jobs

import (
	"context"
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
)

/*
WorkflowEngine runs workflows.
*/
type WorkflowEngine interface {
	/*
		MatchEvent offers an event, as JSON, to the live workflows of its
		trigger.
	*/
	MatchEvent(ctx context.Context, event json.RawMessage) error

	/*
		AdvanceRuns advances runs one after another.
	*/
	AdvanceRuns(ctx context.Context, workspaceID ids.UUID, runIDs []ids.UUID) error

	/*
		AdvanceRun moves one run on, or wakes it when its timer is due.
	*/
	AdvanceRun(ctx context.Context, args AdvanceWorkflowRunArgs) error

	/*
		ResumeOnMessage hands a customer message to the runs of its
		conversation that wait for one.
	*/
	ResumeOnMessage(ctx context.Context, workspaceID, conversationID, messageID ids.UUID) error
}

/*
MatchWorkflowsArgs carries an event to match, as the engine encodes it.
*/
type MatchWorkflowsArgs struct {
	Event json.RawMessage `json:"event"`
}

/*
Kind names the job.
*/
func (MatchWorkflowsArgs) Kind() string { return "match_workflows" }

/*
MatchWorkflows matches one event.
*/
type MatchWorkflows struct {
	Engine WorkflowEngine
}

/*
Handle matches the event.
*/
func (w *MatchWorkflows) Handle(ctx context.Context, job *queue.Job[MatchWorkflowsArgs]) error {
	return w.Engine.MatchEvent(ctx, job.Args.Event)
}

/*
AdvanceWorkflowRunsArgs names the runs an event started, in list order.
*/
type AdvanceWorkflowRunsArgs struct {
	WorkspaceID ids.UUID   `json:"workspace_id"`
	RunIDs      []ids.UUID `json:"run_ids"`
}

/*
Kind names the job.
*/
func (AdvanceWorkflowRunsArgs) Kind() string { return "advance_workflow_runs" }

/*
AdvanceWorkflowRuns advances the runs an event started.
*/
type AdvanceWorkflowRuns struct {
	Engine WorkflowEngine
}

/*
Handle advances the runs in order.
*/
func (w *AdvanceWorkflowRuns) Handle(ctx context.Context, job *queue.Job[AdvanceWorkflowRunsArgs]) error {
	return w.Engine.AdvanceRuns(ctx, job.Args.WorkspaceID, job.Args.RunIDs)
}
