/*
|--------------------------------------------------------------------------
| Trash Workspace
|--------------------------------------------------------------------------
|
| A deleted workspace is not gone yet: it waits in the trash for the
| grace period, so an owner who deleted the wrong one, or a team that
| changes its mind, can bring it back. While it waits nobody can work in
| it, and nothing it handed out keeps working: its access tokens are
| revoked and its pending invitations withdrawn, and neither comes back
| with a restore, so a leaked token never outlives the deletion. Every
| owner is told, with the date after which it is gone for good.
|
*/

package workspaces

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
systemName is how mail names the product when it deleted a workspace on
its own, such as one left without members.
*/
const systemName = "dimmi"

/*
TrashedBy is who deleted a workspace: the user and their membership, or
neither when the system did.
*/
type TrashedBy struct {
	User   *models.User
	Member *ids.UUID
}

/*
TrashWorkspace moves a workspace to the trash inside the caller's
transaction.

Rules:
  - The workspace is purged GraceDays after deletion unless restored.
  - Its live access tokens are revoked and its pending invitations
    withdrawn.
  - Every active owner is mailed.

Events: [appevents.WorkspaceDeleted].

Errors: a wrapped database or mail error.
*/
type TrashWorkspace struct {
	Events    *events.Dispatcher
	Clock     clock.Clock
	Mail      *mail.Manager
	GraceDays int
}

/*
Handle trashes the workspace, which the transaction holds locked, and
returns it as it now is.
*/
func (a *TrashWorkspace) Handle(
	ctx context.Context, tx *database.Tx, w models.Workspace, by TrashedBy,
) (models.Workspace, error) {
	q := models.New(tx)
	now := a.Clock.Now()

	params := models.TrashWorkspaceParams{
		ID: w.ID, Now: now, PurgeAfter: now.Add(time.Duration(a.GraceDays) * 24 * time.Hour),
	}
	if by.User != nil {
		params.DeletedBy = &by.User.ID
	}

	trashed, err := q.TrashWorkspace(ctx, params)
	if err != nil {
		return trashed, fmt.Errorf("trash workspace: %w", err)
	}

	if err := endAccess(ctx, q, trashed, by.Member); err != nil {
		return trashed, err
	}

	err = a.Events.Dispatch(ctx, tx, appevents.WorkspaceDeleted{
		WorkspaceID: w.ID, DeletedBy: deleterID(by), PurgeAfter: *trashed.PurgeAfter,
	})
	if err != nil {
		return trashed, err
	}

	return trashed, a.tellOwners(ctx, tx, trashed, by)
}

/*
endAccess revokes the trashed workspace's tokens and withdraws its
pending invitations, as of its deletion.
*/
func endAccess(
	ctx context.Context, q *models.Queries, w models.Workspace, by *ids.UUID,
) error {
	_, err := q.RevokeWorkspaceTokens(ctx, models.RevokeWorkspaceTokensParams{
		WorkspaceID: w.ID, RevokedBy: by, Now: *w.DeletedAt,
	})
	if err != nil {
		return fmt.Errorf("revoke workspace tokens: %w", err)
	}

	_, err = q.RevokePendingInvitations(ctx, models.RevokePendingInvitationsParams{
		WorkspaceID: w.ID, RevokedBy: by, Now: *w.DeletedAt,
	})
	if err != nil {
		return fmt.Errorf("withdraw pending invitations: %w", err)
	}

	return nil
}

/*
tellOwners mails every active owner that the workspace was deleted and
until when it can be restored.
*/
func (a *TrashWorkspace) tellOwners(ctx context.Context, tx *database.Tx, w models.Workspace, by TrashedBy) error {
	owners, err := models.New(tx).ListActiveOwners(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("list owners: %w", err)
	}

	deletedBy := systemName
	if by.User != nil {
		deletedBy = by.User.Name
	}

	for _, owner := range owners {
		err := appmail.ToUser(a.Mail, owner.User).QueueTx(ctx, tx, appmail.WorkspaceDeletionScheduledMail{
			Name: owner.User.Name, Workspace: w.Name, DeletedBy: deletedBy, PurgeAfter: *w.PurgeAfter,
		})
		if err != nil {
			return fmt.Errorf("mail owner: %w", err)
		}
	}

	return nil
}

/*
deleterID returns the id of the user who deleted the workspace, or
ids.Nil() for the system.
*/
func deleterID(by TrashedBy) ids.UUID {
	if by.User == nil {
		return ids.Nil()
	}

	return by.User.ID
}
