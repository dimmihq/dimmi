/*
|--------------------------------------------------------------------------
| Restore Workspace
|--------------------------------------------------------------------------
|
| Any owner may take a workspace out of the trash until its grace
| period ends, through their own browser or app and having proved who
| they are within the last few minutes. To anyone else a workspace in
| the trash does not exist, so asking about one never confirms it is
| there. Once its purge has begun, nothing can bring it back.
|
| What the deletion ended stays ended: tokens and invitations revoked
| then are not revived, so restoring never hands access back to someone
| who held it before.
|
*/

package workspaces

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RestoreWorkspaceInput is who restores the workspace and which one.
*/
type RestoreWorkspaceInput struct {
	Principal   *appauth.Principal
	WorkspaceID ids.UUID
}

/*
RestoreWorkspace takes a workspace out of the trash.

Rules:
  - Only an active owner finds the workspace; anyone else is told it
    does not exist.
  - The caller confirmed who they are within the confirmation window.
  - A workspace not in the trash is answered as it is.
  - A workspace whose purge began, or whose grace period ended, cannot
    be restored.
  - Every active owner is mailed.

Events: [appevents.WorkspaceRestored].

Errors: [exceptions.ErrWorkspaceNotFound],
[fauth.ErrConfirmationRequired], [exceptions.ErrWorkspacePurging], or a
wrapped error.
*/
type RestoreWorkspace struct {
	DB            *database.DB
	Events        *events.Dispatcher
	Clock         clock.Clock
	Mail          *mail.Manager
	Links         appauth.Links
	ConfirmWindow time.Duration
}

/*
Handle restores the workspace and returns it as it now is.
*/
func (a *RestoreWorkspace) Handle(ctx context.Context, in RestoreWorkspaceInput) (models.Workspace, error) {
	var restored models.Workspace

	scoped := tenancy.With(ctx, in.WorkspaceID.String())
	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		w, member, err := a.ownWorkspace(ctx, q, in)
		if err != nil {
			return err
		}

		if !fauth.Confirmed(in.Principal, a.Clock.Now(), a.ConfirmWindow) {
			return fauth.ErrConfirmationRequired
		}

		restored = w
		if w.DeletedAt == nil {
			return nil
		}

		if purgeStarted(w, a.Clock.Now()) {
			return exceptions.ErrWorkspacePurging
		}

		restored, err = q.RestoreWorkspace(ctx, models.RestoreWorkspaceParams{ID: w.ID, Now: a.Clock.Now()})
		if err != nil {
			return fmt.Errorf("restore workspace: %w", err)
		}

		err = a.Events.Dispatch(ctx, tx, appevents.WorkspaceRestored{WorkspaceID: w.ID, RestoredBy: member.ID})
		if err != nil {
			return err
		}

		return a.tellOwners(ctx, tx, restored, in.Principal.User.Name)
	})

	return restored, err
}

/*
ownWorkspace locks the workspace and returns it with the caller's
membership, when the caller is one of its active owners.
*/
func (a *RestoreWorkspace) ownWorkspace(
	ctx context.Context, q *models.Queries, in RestoreWorkspaceInput,
) (models.Workspace, models.WorkspaceMember, error) {
	w, err := q.LockWorkspace(ctx, in.WorkspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return w, models.WorkspaceMember{}, exceptions.ErrWorkspaceNotFound
	}

	if err != nil {
		return w, models.WorkspaceMember{}, fmt.Errorf("lock workspace: %w", err)
	}

	member, err := q.FindWorkspaceMemberForUser(ctx, models.FindWorkspaceMemberForUserParams{
		WorkspaceID: w.ID, UserID: in.Principal.User.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return w, member, exceptions.ErrWorkspaceNotFound
	}

	if err != nil {
		return w, member, fmt.Errorf("find membership: %w", err)
	}

	owns := slices.Contains(appauth.RolePermissions(enums.Role(member.Role)), "workspace:delete")
	if !owns || member.Status != string(enums.MemberStatusActive) {
		return w, member, exceptions.ErrWorkspaceNotFound
	}

	return w, member, nil
}

/*
tellOwners mails every active owner that the workspace is back.
*/
func (a *RestoreWorkspace) tellOwners(ctx context.Context, tx *database.Tx, w models.Workspace, by string) error {
	owners, err := models.New(tx).ListActiveOwners(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("list owners: %w", err)
	}

	for _, owner := range owners {
		err := appmail.ToUser(a.Mail, owner.User).QueueTx(ctx, tx, appmail.WorkspaceRestoredMail{
			Name: owner.User.Name, Workspace: w.Name, RestoredBy: by, AppURL: a.Links.App(),
		})
		if err != nil {
			return fmt.Errorf("mail owner: %w", err)
		}
	}

	return nil
}

/*
purgeStarted reports whether the workspace's purge began, or its grace
period ended so that it may begin at any moment.
*/
func purgeStarted(w models.Workspace, now time.Time) bool {
	if w.PurgeAfter != nil && !now.Before(*w.PurgeAfter) {
		return true
	}

	var stored struct {
		PurgeStartedAt *time.Time `json:"purge_started_at"`
	}

	if err := json.Unmarshal(w.Settings, &stored); err != nil {
		return true
	}

	return stored.PurgeStartedAt != nil
}
