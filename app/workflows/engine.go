/*
|--------------------------------------------------------------------------
| Workflow Engine
|--------------------------------------------------------------------------
|
| The engine runs published workflows on conversations. An event finds
| the live workflows of its trigger, each is checked in list order, and
| the ones that pass start a run. A run walks its version's steps one
| at a time, each in the same transaction as the cursor that records
| it, so an effect happens exactly once however often a job retries.
| Only one run advances per conversation at a time, so two workflows
| never interleave their changes.
|
| A run that waits, for an answer or for time, simply stops; the event
| or the timer that resumes it finds it by its cursor.
|
*/

package workflows

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
)

/*
Limits bound what runs may do. Steps a run may execute in all and per
job, steps all runs of a conversation may execute per hour, and
messages workflows may send a conversation per hour.
*/
type Limits struct {
	StepsPerRun              int
	StepsPerJob              int
	StepsPerConversationHour int
	MessagesPerHour          int
	RunLifetime              time.Duration
}

/*
DefaultLimits are the limits a workspace runs with.
*/
func DefaultLimits() Limits {
	return Limits{
		StepsPerRun: 200, StepsPerJob: 25, StepsPerConversationHour: 300, MessagesPerHour: 20,
		RunLifetime: 30 * 24 * time.Hour,
	}
}

/*
Engine runs workflows. Delivery hands workflow messages to channels;
IDs reads the public ids definitions name; AgentOn says this server
runs the agent at all, so a step that lets dimmi handle a conversation
hands it over at once when it cannot.
*/
type Engine struct {
	DB       *database.DB
	Events   *events.Dispatcher
	Clock    clock.Clock
	Queue    *queue.Queue
	IDs      *ids.Registry
	Delivery conversations.Delivery
	Limits   Limits
	AgentOn  bool
	live     *liveCache
}

/*
New returns an engine with an empty live cache.
*/
func New(e Engine) *Engine {
	e.live = newLiveCache()
	if e.Limits.StepsPerRun == 0 {
		e.Limits = DefaultLimits()
	}

	return &e
}

/*
WorkflowsChanged drops the cached live workflows of the workspace.
*/
func (e *Engine) WorkflowsChanged(_ context.Context, workspace ids.UUID) {
	e.live.drop(workspace)
}

/*
transaction runs fn in a transaction scoped to the workspace, so row
level security sees the workspace's rows.
*/
func (e *Engine) transaction(ctx context.Context, workspace ids.UUID, fn func(context.Context, *database.Tx) error,
) error {
	return e.DB.Transaction(tenancy.With(ctx, workspace.String()), fn)
}
