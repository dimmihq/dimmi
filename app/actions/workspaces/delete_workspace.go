/*
|--------------------------------------------------------------------------
| Delete Workspace
|--------------------------------------------------------------------------
|
| Deleting a workspace takes everyone's work away, so only an owner may,
| through their own browser or app, having proved who they are within
| the last few minutes, and typing the workspace's address to show they
| mean this one. A leaked token or an unattended laptop is never enough.
| The workspace goes to the trash, where any owner can restore it until
| the grace period ends.
|
*/

package workspaces

import (
	"context"
	"fmt"
	"strings"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteWorkspaceInput is who deletes the workspace, through their
membership of it, and the address they typed to confirm.
*/
type DeleteWorkspaceInput struct {
	Principal   *appauth.Principal
	Membership  authz.Membership
	ConfirmSlug string
}

/*
DeleteWorkspace moves the workspace in play to the trash.

Rules:
  - Only the user's own browser or app may delete a workspace.
  - The caller needs workspace:delete, which only owners hold.
  - The caller confirmed who they are within the confirmation window.
  - The confirmation spells the workspace's slug, case aside.

Events: those of [TrashWorkspace].

Errors: [exceptions.ErrFirstPartyOnly], [exceptions.ErrForbidden],
[fauth.ErrConfirmationRequired], [exceptions.ErrConfirmSlugMismatch], or
a wrapped error.
*/
type DeleteWorkspace struct {
	DB            *database.DB
	Clock         clock.Clock
	ConfirmWindow time.Duration
	Trash         *TrashWorkspace
}

/*
Handle deletes the workspace and returns it as it now is, in the trash.
*/
func (a *DeleteWorkspace) Handle(ctx context.Context, in DeleteWorkspaceInput) (models.Workspace, error) {
	if !in.Principal.IsFirstParty() {
		return models.Workspace{}, exceptions.ErrFirstPartyOnly
	}

	if !in.Membership.Can("workspace:delete") {
		return models.Workspace{}, exceptions.ErrForbidden
	}

	if !fauth.Confirmed(in.Principal, a.Clock.Now(), a.ConfirmWindow) {
		return models.Workspace{}, fauth.ErrConfirmationRequired
	}

	var trashed models.Workspace

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		current, err := models.New(tx).LockWorkspace(ctx, in.Membership.Workspace.ID)
		if err != nil {
			return fmt.Errorf("lock workspace: %w", err)
		}

		if !strings.EqualFold(strings.TrimSpace(in.ConfirmSlug), current.Slug) {
			return exceptions.ErrConfirmSlugMismatch
		}

		trashed, err = a.Trash.Handle(ctx, tx, current, TrashedBy{
			User: &in.Principal.User, Member: &in.Membership.Member.ID,
		})

		return err
	})

	return trashed, err
}
