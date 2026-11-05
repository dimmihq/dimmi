/*
|--------------------------------------------------------------------------
| Delete Inbox
|--------------------------------------------------------------------------
|
| An inbox can go once nothing depends on it: its conversations and its
| channels move to another inbox in the same step, so mail and messages
| that keep arriving land there, and waiting conversations follow that
| inbox's assignment rules from its next drain. The default inbox is
| where everything without a home arrives and stays.
|
*/

package inboxes

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteInboxInput is who deletes which inbox, and where its
conversations go, nil when it has none.
*/
type DeleteInboxInput struct {
	Membership authz.Membership
	InboxID    ids.UUID
	MoveTo     *ids.UUID
}

/*
DeleteInbox takes an inbox out of use.

Rules:
  - The caller needs inboxes:manage.
  - The workspace's default inbox is never deleted.
  - An inbox holding conversations needs another live inbox to move
    them and its channels to.

Events: [appevents.InboxDeleted].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrDefaultInboxUndeletable], [exceptions.ErrMoveToRequired],
[exceptions.ErrMoveToInvalid], or a wrapped database error.
*/
type DeleteInbox struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle deletes the inbox.
*/
func (a *DeleteInbox) Handle(ctx context.Context, in DeleteInboxInput) error {
	if !in.Membership.Can("inboxes:manage") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		ws := in.Membership.Workspace.ID

		inbox, err := find(ctx, q, ws, in.InboxID)
		if err != nil {
			return err
		}

		if err := refuseDefault(ctx, q, ws, inbox.ID); err != nil {
			return err
		}

		now := a.Clock.Now()

		moved, err := a.moveOut(ctx, q, inbox, in.MoveTo)
		if err != nil {
			return err
		}

		if err := q.DeleteInbox(ctx, models.DeleteInboxParams{Now: &now, WorkspaceID: ws, ID: inbox.ID}); err != nil {
			return fmt.Errorf("delete inbox: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.InboxDeleted{
			WorkspaceID: ws, InboxID: inbox.ID, ActorID: in.Membership.Member.ID, MovedTo: in.MoveTo, Moved: moved,
		})
	})
}

/*
refuseDefault refuses deleting the workspace's default inbox.
*/
func refuseDefault(ctx context.Context, q *models.Queries, workspaceID, inboxID ids.UUID) error {
	def, err := q.FindDefaultInbox(ctx, workspaceID)
	if err == nil && def.ID == inboxID {
		return exceptions.ErrDefaultInboxUndeletable
	}

	return nil
}

/*
moveOut moves the inbox's conversations and channels to the target and
returns how many conversations moved. An empty inbox needs no target.
*/
func (a *DeleteInbox) moveOut(
	ctx context.Context, q *models.Queries, inbox models.Inbox, to *ids.UUID,
) (int64, error) {
	held, err := q.CountInboxConversations(ctx, models.CountInboxConversationsParams{
		WorkspaceID: inbox.WorkspaceID, InboxID: inbox.ID,
	})
	if err != nil {
		return 0, fmt.Errorf("count inbox conversations: %w", err)
	}

	if to == nil {
		if held > 0 {
			return 0, exceptions.ErrMoveToRequired
		}

		return 0, nil
	}

	if *to == inbox.ID {
		return 0, exceptions.ErrMoveToInvalid
	}

	if _, err := find(ctx, q, inbox.WorkspaceID, *to); err != nil {
		return 0, exceptions.ErrMoveToInvalid
	}

	now := a.Clock.Now()

	moved, err := q.MoveInboxConversations(ctx, models.MoveInboxConversationsParams{
		ToInboxID: *to, Now: now, WorkspaceID: inbox.WorkspaceID, FromInboxID: inbox.ID,
	})
	if err != nil {
		return 0, fmt.Errorf("move inbox conversations: %w", err)
	}

	err = q.MoveInboxConnections(ctx, models.MoveInboxConnectionsParams{
		ToInboxID: *to, Now: now, WorkspaceID: inbox.WorkspaceID, FromInboxID: inbox.ID,
	})
	if err != nil {
		return 0, fmt.Errorf("move inbox channels: %w", err)
	}

	return moved, nil
}
