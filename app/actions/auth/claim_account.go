/*
|--------------------------------------------------------------------------
| Claim Account
|--------------------------------------------------------------------------
|
| Anyone can sign up with an address they do not own, set a password,
| link a provider, enroll a second factor and create a workspace, then
| wait for the real owner to arrive. When the owner proves the inbox, by
| a mailed link, a reset or a provider that vouches for the address, the
| account is theirs and nothing the stranger set up may survive: the
| password, every session, app grant and access token, every linked
| provider and second factor all end, and the workspaces the account
| holds are let go of. The owner is told to set a password of their own.
|
| A browser already signed in to the account is the exception: whoever
| holds that session registered the account, and proving the inbox from
| there shows the registrant and the owner are the same person, so
| nothing is claimed from anyone.
|
| Claiming happens inside the transaction that proves the inbox, so the
| account is never verified with the stranger's access still alive.
|
*/

package auth

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/session"

	"github.com/dimmihq/dimmi/app/actions/members"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ClaimAccount hands an unverified account to the owner of its inbox, who
just proved it.

Rules:
  - Runs in the caller's transaction, before the email is marked
    proven.
  - Removes the password, ends every browser session, first-party app
    grant and access token of the user, unlinks every provider, and
    removes every second factor.
  - Lets go of the account's workspaces: see
    [members.ReleaseMemberships].
  - Logs what ended and, when a password was removed, mails the owner.

Events: [appevents.UserSignedOutEverywhere], and those of
[members.ReleaseMemberships].

Errors: a wrapped error.
*/
type ClaimAccount struct {
	Clock       clock.Clock
	Events      *events.Dispatcher
	Mail        *mail.Manager
	Links       appauth.Links
	Sessions    *session.Manager
	AppGrants   appauth.AppGrants
	SecurityLog *appauth.SecurityLog
	Memberships *members.ReleaseMemberships
}

/*
Prove marks the account's address proven, claiming the account from
whoever registered it unless the browser is already signed in to it,
and returns the account as it now is. An account already proven is left
alone.
*/
func (a *ClaimAccount) Prove(
	ctx context.Context, tx *database.Tx, user models.User, client appauth.Client,
) (models.User, error) {
	if user.IsVerified() {
		return user, nil
	}

	if !signedInAs(ctx, user) {
		if err := a.Handle(ctx, tx, user, client); err != nil {
			return models.User{}, err
		}

		user.PasswordHash, user.TwoFactorConfirmedAt = nil, nil
	}

	now := a.Clock.Now()

	err := models.New(tx).MarkUserEmailVerified(ctx, models.MarkUserEmailVerifiedParams{ID: user.ID, Now: now})
	if err != nil {
		return models.User{}, fmt.Errorf("mark email verified: %w", err)
	}

	user.EmailVerifiedAt = &now

	return user, nil
}

/*
Handle claims the account inside the transaction of the proof, which
the context carries too, and tells the owner their password is gone.
*/
func (a *ClaimAccount) Handle(ctx context.Context, tx *database.Tx, user models.User, client appauth.Client) error {
	if err := a.Wipe(ctx, tx, user, client); err != nil {
		return err
	}

	err := a.Events.Dispatch(ctx, tx, appevents.UserSignedOutEverywhere{UserID: user.ID})
	if err != nil || !user.HasPassword() {
		return err
	}

	return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.PasswordChangedMail{
		Name: user.Name, Reason: appmail.PasswordWasRemoved, SecurityURL: a.Links.Security(),
	})
}

/*
Wipe removes everything a stranger may have set up on the account and
logs it. A caller that sets a new password right after tells the owner
about that instead.
*/
func (a *ClaimAccount) Wipe(ctx context.Context, tx *database.Tx, user models.User, client appauth.Client) error {
	now := a.Clock.Now()
	q := models.New(tx)

	if err := q.UpdateUserPassword(ctx, models.UpdateUserPasswordParams{ID: user.ID, Now: now}); err != nil {
		return fmt.Errorf("remove unproven password: %w", err)
	}

	sessions, err := a.Sessions.RevokeOthers(ctx, user.ID, ids.Nil())
	if err != nil {
		return fmt.Errorf("end sessions of claimed account: %w", err)
	}

	err = a.AppGrants.RevokeUserAppGrants(ctx, user.ID, appauth.RevokedForPreHijackDefense, ids.Nil())
	if err != nil {
		return fmt.Errorf("end app grants of claimed account: %w", err)
	}

	tokens, err := q.RevokeUserTokens(ctx, models.RevokeUserTokensParams{UserID: user.ID, Now: now})
	if err != nil {
		return fmt.Errorf("revoke tokens of claimed account: %w", err)
	}

	identities, factors, err := a.removeWaysIn(ctx, q, user)
	if err != nil {
		return err
	}

	released, err := a.Memberships.Handle(ctx, tx, user.ID)
	if err != nil {
		return err
	}

	return a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: user.ID, Type: appauth.EventSessionsRevokedOthers, Client: client,
		Metadata: map[string]any{
			"reason": appauth.RevokedForPreHijackDefense, "sessions_ended": sessions, "tokens_revoked": tokens,
			"identities_removed": identities, "factors_removed": factors,
			"workspaces_trashed": released.Trashed, "memberships_left": released.Left,
		},
	})
}

/*
removeWaysIn unlinks every provider and removes every second factor of
the user, and returns how many of each there were.
*/
func (a *ClaimAccount) removeWaysIn(ctx context.Context, q *models.Queries, user models.User) (int64, int64, error) {
	identities, err := q.DeleteUserOAuthIdentities(ctx, user.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("unlink providers of claimed account: %w", err)
	}

	factors, err := q.DeleteUserAuthFactors(ctx, user.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("remove factors of claimed account: %w", err)
	}

	err = q.UpdateUserTwoFactorConfirmedAt(ctx, models.UpdateUserTwoFactorConfirmedAtParams{
		ID: user.ID, Now: a.Clock.Now(),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("turn off two-factor of claimed account: %w", err)
	}

	return identities, factors, nil
}

/*
signedInAs reports whether the browser making the request holds a
session of the user.
*/
func signedInAs(ctx context.Context, user models.User) bool {
	s, ok := session.From(ctx)
	if !ok {
		return false
	}

	current, ok := s.UserID()

	return ok && current == user.ID
}
