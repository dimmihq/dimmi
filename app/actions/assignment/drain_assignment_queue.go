/*
|--------------------------------------------------------------------------
| Drain Assignment Queue
|--------------------------------------------------------------------------
|
| When someone frees up, by closing a conversation, coming back online or
| getting a higher limit, the conversations waiting in their workspace get
| another chance, each inbox's in its own queue order. Each conversation
| is taken in a transaction of its own and skipped when another drain
| holds it, so two workers never assign the same one and a failure costs
| only the conversation it happened on.
|
*/

package assignment

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
drainLimit is the most conversations one drain looks at per inbox, so a
long queue is worked through over several runs.
*/
const drainLimit = 500

/*
DrainAssignmentQueue assigns the waiting conversations of a workspace
that someone can now take.

Rules:
  - Inboxes are drained one after another, each in its queue order.
  - A conversation still nobody can take keeps waiting; the drain goes
    on with the next, which may belong to another pool.

Events: those of [AssignOnArrival].

Errors: a wrapped database error.
*/
type DrainAssignmentQueue struct {
	Arrival *AssignOnArrival
}

/*
Handle drains the workspace and returns how many conversations were
assigned.
*/
func (a *DrainAssignmentQueue) Handle(ctx context.Context, workspaceID ids.UUID) (int, error) {
	scoped := tenancy.With(ctx, workspaceID.String())

	var inboxes []ids.UUID

	err := a.Arrival.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error
		inboxes, err = models.New(tx).ListWaitingInboxes(ctx, workspaceID)

		return err
	})
	if err != nil {
		return 0, fmt.Errorf("list waiting inboxes: %w", err)
	}

	assigned := 0

	for _, inbox := range inboxes {
		n, err := a.drainInbox(scoped, workspaceID, inbox)
		assigned += n

		if err != nil {
			return assigned, err
		}
	}

	return assigned, nil
}

/*
drainInbox works through the inbox's waiting conversations once.
*/
func (a *DrainAssignmentQueue) drainInbox(ctx context.Context, workspaceID, inboxID ids.UUID) (int, error) {
	tried := []ids.UUID{}
	assigned := 0

	for range drainLimit {
		res, id, err := a.next(ctx, workspaceID, inboxID, tried)
		if err != nil || id == ids.Nil() {
			return assigned, err
		}

		tried = append(tried, id)

		if res.MemberID != nil {
			assigned++
		}
	}

	return assigned, nil
}

/*
next takes the inbox's next waiting conversation and tries to assign
it, returning ids.Nil() when none is left.
*/
func (a *DrainAssignmentQueue) next(
	ctx context.Context, workspaceID, inboxID ids.UUID, tried []ids.UUID,
) (PickResult, ids.UUID, error) {
	var (
		res PickResult
		id  ids.UUID
	)

	err := a.Arrival.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: workspaceID, ID: inboxID})
		if err != nil {
			return fmt.Errorf("find inbox: %w", err)
		}

		c, err := q.FindNextWaitingConversation(ctx, models.FindNextWaitingConversationParams{
			WorkspaceID: workspaceID, InboxID: inboxID, Tried: tried,
			ByPriority: inbox.ReadSettings().Assignment.QueueOrder == models.QueuePriorityThenOldest,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find waiting conversation: %w", err)
		}

		id = c.ID
		res, err = a.Arrival.assign(ctx, a.Arrival.writer(tx), c, ReasonDrain)

		return err
	})

	return res, id, err
}

/*
WaitingWorkspaces returns the workspaces with conversations waiting for
an assignee, at most limit.
*/
func WaitingWorkspaces(ctx context.Context, db *database.DB, limit int32) ([]ids.UUID, error) {
	found, err := models.New(db).ListAssignmentWaitingWorkspaces(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list waiting workspaces: %w", err)
	}

	return found, nil
}
