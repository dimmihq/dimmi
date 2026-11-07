/*
|--------------------------------------------------------------------------
| Starting Runs
|--------------------------------------------------------------------------
|
| A run starts on the workflow's live version and remembers the event
| that started it, so a retried event never starts a second run. A
| customer-facing run takes hold of the conversation in the same
| transaction, which also keeps the automatic agent out of it. The runs
| an event starts advance from one job, one after another in list
| order, each in a transaction of its own.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
start creates the candidate's run, and reports false when the event
already started one.
*/
func (e *Engine) start(ctx context.Context, q *models.Queries, m *match, c candidate) (bool, error) {
	now := e.Clock.Now()
	data, err := json.Marshal(m.subject.trigger)
	if err != nil || m.subject.trigger == nil {
		data = json.RawMessage("{}")
	}

	cursor, chain := c.index.First(), m.ev.Chain
	if chain == nil {
		chain = []ids.UUID{}
	}

	run, err := q.CreateWorkflowRun(ctx, models.CreateWorkflowRunParams{
		ID: ids.NewV7At(now), WorkspaceID: m.conv.WorkspaceID, WorkflowID: c.workflowID, VersionID: c.versionID,
		ConversationID: &m.conv.ID, ContactID: &m.conv.ContactID, Kind: c.kind, Mode: "live", Trigger: m.ev.Trigger,
		TriggerEventID: eventID(m.ev.EventID), TriggerData: data, Chain: chain, CursorStepID: &cursor,
		Vars: json.RawMessage("{}"), Now: now,
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("start workflow run: %w", err)
	}

	if c.kind == workflowspec.KindCustomerFacing {
		claimed, err := q.ClaimConversationForRun(ctx, models.ClaimConversationForRunParams{
			RunID: &run.ID, Now: now, WorkspaceID: m.conv.WorkspaceID, ConversationID: m.conv.ID,
		})
		if err != nil {
			return false, fmt.Errorf("claim conversation: %w", err)
		}

		if claimed == 0 {
			return false, nil
		}

		m.facing, m.claimed = true, true
	}

	m.started = append(m.started, run.ID)
	m.cut = m.cut || c.spec.Exclusive

	return true, nil
}

/*
eventID returns the event id for the idempotency column, nil for none.
*/
func eventID(id string) *string {
	if id == "" {
		return nil
	}

	return &id
}

/*
queueBatch queues the job that advances the event's runs in order.
*/
func (e *Engine) queueBatch(ctx context.Context, tx *database.Tx, workspace ids.UUID, runs []ids.UUID) error {
	if err := e.Queue.PushTx(ctx, tx, jobs.AdvanceWorkflowRunsArgs{WorkspaceID: workspace, RunIDs: runs}); err != nil {
		return fmt.Errorf("queue workflow runs: %w", err)
	}

	return nil
}

/*
AdvanceBatch advances runs one after another.
*/
func (e *Engine) AdvanceBatch(ctx context.Context, workspace ids.UUID, runs []ids.UUID) error {
	for _, run := range runs {
		if err := e.Advance(ctx, Step{WorkspaceID: workspace, RunID: run}); err != nil {
			return err
		}
	}

	return nil
}
