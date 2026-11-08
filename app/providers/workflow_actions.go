/*
|--------------------------------------------------------------------------
| Workflow Wiring
|--------------------------------------------------------------------------
|
| The actions behind a workspace's workflows and the engine that runs
| them: the engine hears the domain events that trigger, answer and
| interrupt runs, and its jobs advance them.
|
*/

package providers

import (
	"errors"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/procedures"
	"github.com/dimmihq/dimmi/app/actions/schedules"
	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/jobs"
	engine "github.com/dimmihq/dimmi/app/workflows"
)

/*
bootWorkflows builds the workflow actions and the engine, and wires the
engine to the events it runs on.
*/
func bootWorkflows(a *app.App) {
	runner := engine.New(engine.Engine{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Queue: a.Queue, IDs: a.IDs,
		Delivery: conversations.Delivery{Channels: a.Channels, Queue: a.Queue}, AgentOn: a.Config.AI.Enabled,
	})
	runner.Listen(a.Events)
	a.ContactErasers.Register(engine.RunEraser{Engine: runner})

	a.Actions.WorkflowEngine = runner
	a.Actions.Workflows = &workflows.Workflows{DB: a.DB, Clock: a.Clock, IDs: a.IDs, Live: runner}
	a.Actions.Schedules = &schedules.Schedules{DB: a.DB, Clock: a.Clock, IDs: a.IDs}
	a.Actions.Procedures = &procedures.Procedures{DB: a.DB, Clock: a.Clock, IDs: a.IDs}
	a.Actions.Audiences = &audiences.Audiences{DB: a.DB, Clock: a.Clock, IDs: a.IDs}
}

/*
registerWorkflowJobs registers the engine's jobs.
*/
func registerWorkflowJobs(a *app.App) error {
	runner := a.Actions.WorkflowEngine
	if runner == nil {
		return nil
	}

	return errors.Join(
		queue.Register(a.Queue, &jobs.MatchWorkflows{Engine: runner}),
		queue.Register(a.Queue, &jobs.AdvanceWorkflowRuns{Engine: runner}),
		queue.Register(a.Queue, &jobs.AdvanceWorkflowRun{Engine: runner}),
		queue.Register(a.Queue, &jobs.ResumeWorkflowInput{Engine: runner}),
		queue.Register(a.Queue, &jobs.ResumeWorkflowOnAgent{Engine: runner}),
		queue.Register(a.Queue, &jobs.ArmQuietTriggers{Engine: runner}),
		queue.Register(a.Queue, &jobs.FireQuietTrigger{Engine: runner}),
	)
}
