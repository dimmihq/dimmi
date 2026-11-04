/*
|--------------------------------------------------------------------------
| Ensure Workspace Has Owner
|--------------------------------------------------------------------------
|
| Every change to members keeps an active owner, yet a workspace can
| still end up without one, such as when its last owner deletes their
| account. Someone must be able to run it, so the longest-standing
| admin becomes owner, or the longest-standing member when there is no
| admin, and is told why. A workspace nobody is active in any more goes
| to the trash, where it waits out the grace period like any other.
|
*/

package members

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OwnerRepair is what EnsureWorkspaceHasOwner did to a workspace.
*/
type OwnerRepair string

/*
The outcomes of EnsureWorkspaceHasOwner: the workspace had an owner or
is in the trash already, a member was promoted, or the workspace had
nobody left and went to the trash.
*/
const (
	OwnerKept     OwnerRepair = "kept"
	OwnerPromoted OwnerRepair = "promoted"
	OwnerTrashed  OwnerRepair = "trashed"
)

/*
EnsureWorkspaceHasOwner gives a live workspace without an active owner
one.

Rules:
  - A workspace in the trash, or with an active owner, is left alone.
  - The longest-standing active admin becomes owner, else the
    longest-standing active member, who is mailed.
  - A workspace without active members goes to the trash.

Events: [appevents.MemberRoleChanged], changed by nobody, or those of
[workspaces.TrashWorkspace].

Errors: a wrapped database or mail error.
*/
type EnsureWorkspaceHasOwner struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	Mail   *mail.Manager
	Links  appauth.Links
	Trash  *workspaces.TrashWorkspace
}

/*
Handle checks the workspace and repairs it when it has no owner.
*/
func (a *EnsureWorkspaceHasOwner) Handle(ctx context.Context, workspaceID ids.UUID) (OwnerRepair, error) {
	outcome := OwnerKept

	scoped := tenancy.With(ctx, workspaceID.String())
	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		w, err := q.LockWorkspace(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		owners, err := q.CountActiveOwners(ctx, workspaceID)
		if err != nil || owners > 0 || w.DeletedAt != nil {
			return err
		}

		heir, err := oldestActive(ctx, q, workspaceID)
		if errors.Is(err, database.ErrNotFound) {
			outcome = OwnerTrashed
			_, err = a.Trash.Handle(ctx, tx, w, workspaces.TrashedBy{})

			return err
		}

		if err != nil {
			return err
		}

		outcome = OwnerPromoted

		return a.promote(ctx, tx, w, heir)
	})

	return outcome, err
}

/*
promote makes the member an owner, announced as a change by nobody,
and mails them.
*/
func (a *EnsureWorkspaceHasOwner) promote(
	ctx context.Context, tx *database.Tx, w models.Workspace, heir models.WorkspaceMember,
) error {
	writer := memberWriter{tx: tx, events: a.Events, by: ids.Nil(), now: a.Clock.Now()}
	if _, err := writer.changeRole(ctx, heir, appauth.OwnerRole()); err != nil {
		return err
	}

	user, err := models.New(tx).FindUser(ctx, heir.UserID)
	if err != nil {
		return fmt.Errorf("load new owner: %w", err)
	}

	return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.OwnerPromotedBySystemMail{
		Name: user.Name, Workspace: w.Name, MembersURL: a.Links.Members(),
	})
}

/*
oldestActive returns the longest-standing active admin, or member when
there is no admin, or database.ErrNotFound when nobody is active.
*/
func oldestActive(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (models.WorkspaceMember, error) {
	for _, role := range []enums.Role{appauth.ManagerRole(), enums.RoleMember} {
		member, err := q.FindOldestActiveMember(ctx, models.FindOldestActiveMemberParams{
			WorkspaceID: workspaceID, Role: string(role),
		})
		if errors.Is(err, database.ErrNotFound) {
			continue
		}

		if err != nil {
			return member, fmt.Errorf("find heir: %w", err)
		}

		return member, nil
	}

	return models.WorkspaceMember{}, database.ErrNotFound
}
