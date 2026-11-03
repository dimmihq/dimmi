/*
|--------------------------------------------------------------------------
| Claim Account
|--------------------------------------------------------------------------
|
| Anyone can sign up with an address they do not own and set a
| password, then wait for the real owner to arrive. When the owner
| proves the inbox, by a mailed link or a provider that vouches for the
| address, the account is theirs, and nothing the stranger set up may
| survive: the unproven password is removed, every session, app grant
| and access token ends, and the owner is told to set a password of
| their own.
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
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ClaimAccount hands an unverified account with a password to the owner
of its inbox, who just proved it.

Rules:
  - Runs in the caller's transaction, before the email is marked
    proven.
  - Removes the password, ends every browser session, first-party app
    grant and access token of the user.
  - Logs the revocation and mails the owner.

Errors: a wrapped error.
*/
type ClaimAccount struct {
	Clock       clock.Clock
	Mail        *mail.Manager
	Links       appauth.Links
	Sessions    *session.Manager
	AppGrants   appauth.AppGrants
	SecurityLog *appauth.SecurityLog
}

/*
Handle claims the account inside the transaction of the proof, which
the context carries too.
*/
func (a *ClaimAccount) Handle(ctx context.Context, tx *database.Tx, user models.User, client appauth.Client) error {
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

	err = a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: user.ID, Type: appauth.EventSessionsRevokedOthers, Client: client,
		Metadata: map[string]any{
			"reason": appauth.RevokedForPreHijackDefense, "sessions_ended": sessions, "tokens_revoked": tokens,
		},
	})
	if err != nil {
		return err
	}

	return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.PasswordChangedMail{
		Name: user.Name, Reason: appmail.PasswordWasRemoved, SecurityURL: a.Links.Security(),
	})
}
