/*
|--------------------------------------------------------------------------
| Leave Every Workspace
|--------------------------------------------------------------------------
|
| What deleting an account means for the workspaces it belongs to, shown
| beforehand and applied in the deletion's transaction, by one rule so the
| preview never promises what the deletion refuses. A workspace the
| person alone owns blocks them while anyone else works there. One they
| are alone in goes to the trash with them. Any other they simply leave,
| and a workspace already in the trash never blocks anything.
|
| Invitations the person sent stay valid, unlike when someone is removed:
| leaving of their own accord says nothing against whoever they invited.
|
*/

package members

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
What deleting the account does to one workspace.
*/
const (
	fateLeave = iota
	fateTrash
	fateBlock
)

/*
DeletedWorkspace names a workspace in the account deletion preview.
*/
type DeletedWorkspace struct {
	ID   ids.UUID
	Name string
	Slug string
}

/*
DeletionPreview is what deleting the account would do: the workspaces
that block it and those that would go to the trash.
*/
type DeletionPreview struct {
	Blocking      []DeletedWorkspace
	WillBeDeleted []DeletedWorkspace
}

/*
PreviewAccountDeletion says what deleting the caller's account would do
to their workspaces, changing nothing.

Errors: a wrapped database error.
*/
type PreviewAccountDeletion struct {
	DB *database.DB
}

/*
Handle reads the user's workspaces.
*/
func (a *PreviewAccountDeletion) Handle(ctx context.Context, userID ids.UUID) (DeletionPreview, error) {
	preview := DeletionPreview{Blocking: []DeletedWorkspace{}, WillBeDeleted: []DeletedWorkspace{}}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListUserActiveMemberships(ctx, userID)
		if err != nil {
			return fmt.Errorf("list memberships to preview: %w", err)
		}

		for _, row := range rows {
			if err := tx.SetTenant(ctx, row.WorkspaceID.String()); err != nil {
				return err
			}

			fate, err := fateOf(tenancy.With(ctx, row.WorkspaceID.String()), q, row)
			if err != nil {
				return err
			}

			preview.add(fate, row)
		}

		return tx.SetTenant(ctx, "")
	})

	return preview, err
}

func (p *DeletionPreview) add(fate int, row models.ListUserActiveMembershipsRow) {
	workspace := DeletedWorkspace{ID: row.WorkspaceID, Name: row.WorkspaceName, Slug: string(row.WorkspaceSlug)}

	switch fate {
	case fateBlock:
		p.Blocking = append(p.Blocking, workspace)
	case fateTrash:
		p.WillBeDeleted = append(p.WillBeDeleted, workspace)
	}
}

/*
fateOf decides what deleting the account does to one workspace.
*/
func fateOf(ctx context.Context, q *models.Queries, row models.ListUserActiveMembershipsRow) (int, error) {
	if row.Trashed || !appauth.IsOwnerRole(enums.Role(row.Role)) {
		return fateLeave, nil
	}

	owners, err := q.CountActiveOwners(ctx, row.WorkspaceID)
	if err != nil {
		return 0, fmt.Errorf("count owners: %w", err)
	}

	if owners > 1 {
		return fateLeave, nil
	}

	active, err := q.CountActiveMembers(ctx, row.WorkspaceID)
	if err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}

	if active > 1 {
		return fateBlock, nil
	}

	return fateTrash, nil
}

/*
LeaveEveryWorkspace ends all of a user's memberships inside the deletion's
transaction.

Events: those of [workspaces.TrashWorkspace], and
[appevents.MemberDeactivated] by the member themselves.

Errors: [exceptions.ErrSoleOwner] naming the blocking workspaces, or a
wrapped error.
*/
type LeaveEveryWorkspace struct {
	Events *events.Dispatcher
	Clock  clock.Clock
	Trash  *workspaces.TrashWorkspace
}

/*
Handle leaves every workspace, one at a time, and leaves the transaction
scoped to no workspace. The member is gone before a workspace is
trashed, so nobody is mailed about a restore they could never make.
*/
func (a *LeaveEveryWorkspace) Handle(ctx context.Context, tx *database.Tx, userID ids.UUID) error {
	q := models.New(tx)

	rows, err := q.ListUserActiveMemberships(ctx, userID)
	if err != nil {
		return fmt.Errorf("list memberships to end: %w", err)
	}

	blocking := []string{}

	for _, row := range rows {
		if err := tx.SetTenant(ctx, row.WorkspaceID.String()); err != nil {
			return err
		}

		scoped := tenancy.With(ctx, row.WorkspaceID.String())

		s, err := lock(scoped, q, row.WorkspaceID, row.MemberID)
		if err != nil {
			return err
		}

		fate, err := fateOf(scoped, q, row)
		if err != nil {
			return err
		}

		if fate == fateBlock {
			blocking = append(blocking, models.PublicID(models.PrefixWorkspace, row.WorkspaceID))

			continue
		}

		if err := a.leave(scoped, tx, s, fate); err != nil {
			return err
		}
	}

	if len(blocking) > 0 {
		return exceptions.ErrSoleOwner.With("workspaces", blocking)
	}

	return tx.SetTenant(ctx, "")
}

/*
leave ends the membership and trashes the workspace when it is the
member's alone.
*/
func (a *LeaveEveryWorkspace) leave(ctx context.Context, tx *database.Tx, s subject, fate int) error {
	w := memberWriter{tx: tx, events: a.Events, by: s.target.ID, now: a.Clock.Now(), keepInvitations: true}
	if err := w.deactivate(ctx, s.target, ids.Nil()); err != nil {
		return err
	}

	if fate != fateTrash {
		return nil
	}

	_, err := a.Trash.Handle(ctx, tx, s.workspace, workspaces.TrashedBy{})

	return err
}
