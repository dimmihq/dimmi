/*
|--------------------------------------------------------------------------
| Disable Two-Factor
|--------------------------------------------------------------------------
|
| Turning two-factor authentication off, whichever way it happens:
| removing the last factor that protects the account, removing them all
| at once, or an operator doing it from the command line for a user who
| lost their phone. Every factor and the recovery codes go, the account
| is marked unprotected, and the owner is told.
|
| The first-party apps signed in under the protection that was just
| removed are signed out, except the one asking. Browser sessions stay:
| the browser asking proved who it is moments ago.
|
| A user working in a workspace that requires a second factor cannot
| turn theirs off: they would lock themselves out of it. They leave the
| workspace, or an admin lifts the requirement, first. Only an operator
| may force it, for a user who lost their phone, who is then kept out of
| such workspaces until they add a new factor.
|
*/

package factors

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DisableTwoFactorInput is the user, locked by the caller, the client,
the first-party app grant to keep, ids.Nil() for none, and whether to
Force it past workspaces that require a second factor.
*/
type DisableTwoFactorInput struct {
	User      models.User
	Client    appauth.Client
	KeepGrant ids.UUID
	Force     bool
}

/*
RequiringWorkspace is a workspace that keeps a user from turning
two-factor authentication off.
*/
type RequiringWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
DisableTwoFactor turns two-factor authentication off for a user, inside
the caller's transaction.

Rules:
  - Unless forced, a user who is an active member of a live workspace
    that requires a second factor keeps theirs.
  - Every factor of the user is deleted, recovery codes included.
  - The account is marked unprotected.
  - First-party app grants end, except the kept one.
  - The change is logged and mailed to the owner.

Errors: [exceptions.ErrTwoFactorRequiredByWorkspace] with the
workspaces, or a wrapped error.
*/
type DisableTwoFactor struct {
	Clock       clock.Clock
	Mail        *mail.Manager
	Links       appauth.Links
	AppGrants   appauth.AppGrants
	SecurityLog *appauth.SecurityLog
}

/*
Handle turns two-factor authentication off within the transaction.
*/
func (a *DisableTwoFactor) Handle(ctx context.Context, tx *database.Tx, in DisableTwoFactorInput) error {
	q := models.New(tx)
	now := a.Clock.Now()

	if !in.Force {
		if err := blockedByWorkspaces(ctx, q, in.User); err != nil {
			return err
		}
	}

	if _, err := q.DeleteUserAuthFactors(ctx, in.User.ID); err != nil {
		return fmt.Errorf("delete factors to turn two-factor off: %w", err)
	}

	err := q.UpdateUserTwoFactorConfirmedAt(ctx, models.UpdateUserTwoFactorConfirmedAtParams{ID: in.User.ID, Now: now})
	if err != nil {
		return fmt.Errorf("turn two-factor authentication off: %w", err)
	}

	err = a.AppGrants.RevokeUserAppGrants(ctx, in.User.ID, appauth.RevokedForTwoFactorDisabled, in.KeepGrant)
	if err != nil {
		return fmt.Errorf("end app grants after two-factor turned off: %w", err)
	}

	err = a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: in.User.ID, Type: appauth.EventTwoFactorDisabled, Client: in.Client,
	})
	if err != nil {
		return err
	}

	err = appmail.ToUser(a.Mail, in.User).QueueTx(ctx, tx, appmail.TwoFactorDisabledMail{
		Name: in.User.Name, SecurityURL: a.Links.Security(),
	})
	if err != nil {
		return fmt.Errorf("queue two-factor disabled mail: %w", err)
	}

	return nil
}

/*
blockedByWorkspaces refuses turning two-factor authentication off for a
user who works in a workspace that requires it, listing those
workspaces.
*/
func blockedByWorkspaces(ctx context.Context, q *models.Queries, user models.User) error {
	rows, err := q.ListWorkspacesRequiringTwoFactor(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("list workspaces requiring two-factor: %w", err)
	}

	if len(rows) == 0 {
		return nil
	}

	requiring := make([]RequiringWorkspace, 0, len(rows))
	for _, row := range rows {
		requiring = append(requiring, RequiringWorkspace{
			ID: models.PublicID(models.PrefixWorkspace, row.ID), Name: row.Name,
		})
	}

	return exceptions.ErrTwoFactorRequiredByWorkspace.With("workspaces", requiring)
}
