/*
|--------------------------------------------------------------------------
| Operator Agent
|--------------------------------------------------------------------------
|
| Operator's wiring, after the agent: it answers with the agent's model
| and spends from the agent's budget, reads and changes the workspace
| through the inbox, help center and procedure actions, and, when a
| workspace asks for it, looks at every conversation a customer starts a
| little after it arrives.
|
*/

package providers

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	aijobs "github.com/dimmihq/dimmi/app/ai/jobs"
	"github.com/dimmihq/dimmi/app/ai/operator"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
triageDelay is how long after a conversation arrives Operator triages
it, so the customer's first messages are there to read.
*/
const triageDelay = 30 * time.Second

/*
bootOperatorAgent wires Operator's engine, actions, listener and job.
*/
func bootOperatorAgent(a *app.App) error {
	engine := newOperatorEngine(a)
	a.Actions.OperatorAgent = &app.OperatorAgentActions{
		Engine:     engine,
		Settings:   &operatorchat.Settings{DB: a.DB, Clock: a.Clock},
		Threads:    &operatorchat.Threads{DB: a.DB, Clock: a.Clock, Operator: engine, Limiter: a.Limiter},
		Changesets: &operatorchat.Changesets{DB: a.DB, Operator: engine},
		Memories:   &operatorchat.Memories{DB: a.DB},
	}
	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.ConversationCreated) error {
		return enqueueTriage(ctx, a, e)
	})
	return queue.Register(a.Queue, &aijobs.TriageWithOperator{Operator: engine})
}

/*
newOperatorEngine builds the engine over the agent's model, prices and
the workspace actions its tools call.
*/
func newOperatorEngine(a *app.App) *operator.Engine {
	inbox := a.Actions.Inbox
	return &operator.Engine{
		DB: a.DB, IDs: a.IDs, Clock: a.Clock, LLM: a.LLM, Prices: a.AI.Prices, Publisher: a.Broadcast,
		Timeouts: a.Config.AI.Timeouts, StorePayloads: a.Config.AI.TracesStorePayloads,
		Services: operator.Services{
			Knowledge: a.Knowledge, Show: inbox.ShowConversation, Search: inbox.Search, Context: inbox.ShowContext,
			Messages: inbox.ListMessages, Runs: a.Actions.AI.ListRuns, Change: inbox.ApplyChanges,
			Note: inbox.AddNote, Delete: inbox.DeleteMessage, Contacts: inbox.UpdateContact,
			Articles: a.Actions.Articles, Procedures: a.Actions.Procedures, Audit: a.Actions.RecordAuditLog,
			Actions: a.AI.Actions,
		},
	}
}

/*
enqueueTriage queues Operator's look at a conversation a customer
started, when the workspace asked for it.
*/
func enqueueTriage(ctx context.Context, a *app.App, e appevents.ConversationCreated) error {
	if e.InitiatedBy != "contact" || !a.Config.AI.Enabled {
		return nil
	}
	var wanted bool
	scoped := tenancy.With(ctx, e.WorkspaceID.String())
	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		s, err := operator.LoadSettings(ctx, models.New(tx), e.WorkspaceID)
		wanted = s.Enabled && s.TriageOnArrival
		return err
	})
	if err != nil || !wanted {
		return err
	}
	return a.Queue.Push(context.WithoutCancel(ctx), aijobs.TriageWithOperatorArgs{
		WorkspaceID: e.WorkspaceID, ConversationID: e.ConversationID,
	}, queue.At(a.Clock.Now().Add(triageDelay)), queue.Unique(queue.UniqueOpts{ByArgs: true}))
}
