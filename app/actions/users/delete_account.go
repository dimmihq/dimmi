/*
|--------------------------------------------------------------------------
| Delete Account
|--------------------------------------------------------------------------
|
| A person may leave for good, at once and irreversibly, after proving
| who they are and typing their address. Nobody else's work may suffer
| for it, so a workspace they alone own with other people in it blocks
| the deletion until it is handed over. Nothing that identifies them
| stays: sign-in details, secrets and the security log are removed, and
| a nameless row keeps what they wrote attributed. Their address is free
| from that moment, for a new account with no link to the old one.
|
*/

package users

import (
	"context"
	"fmt"
	"strings"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/session"

	"github.com/dimmihq/dimmi/app/actions/members"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
deletedName is who a deleted account's old messages are shown as.
*/
const deletedName = "Former teammate"

/*
DeleteAccountInput is the caller and the address they typed to confirm.
*/
type DeleteAccountInput struct {
	Principal    *appauth.Principal
	ConfirmEmail string
}

/*
DeleteAccount deletes the caller's own account.

Rules:
  - Only the user's own browser or app, confirmed within the window.
  - The typed address must be the account's, case aside.
  - A workspace the user alone owns with other active members blocks it.
  - A workspace the user is alone in goes to the trash and is purged when
    the grace period ends, since nobody is left to restore it.
  - Invitations the user sent stay valid.

Events: [appevents.UserDeleted], [appevents.UserSignedOutEverywhere], and
those of [members.LeaveEveryWorkspace].

Errors: [exceptions.ErrFirstPartyOnly], [fauth.ErrConfirmationRequired],
[exceptions.ErrConfirmEmailMismatch], [exceptions.ErrSoleOwner], or a
wrapped error.
*/
type DeleteAccount struct {
	DB            *database.DB
	Clock         clock.Clock
	Events        *events.Dispatcher
	Mail          *mail.Manager
	Queue         *queue.Queue
	Sessions      *session.Manager
	AppGrants     appauth.AppGrants
	Workspaces    *members.LeaveEveryWorkspace
	ConfirmWindow time.Duration
}

/*
Handle deletes the account and ends the browser session asking.
*/
func (a *DeleteAccount) Handle(ctx context.Context, in DeleteAccountInput) error {
	if !in.Principal.IsFirstParty() {
		return exceptions.ErrFirstPartyOnly
	}

	if !fauth.Confirmed(in.Principal, a.Clock.Now(), a.ConfirmWindow) {
		return fauth.ErrConfirmationRequired
	}

	if !strings.EqualFold(strings.TrimSpace(in.ConfirmEmail), in.Principal.User.Email) {
		return exceptions.ErrConfirmEmailMismatch
	}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		user, err := models.New(tx).LockUser(ctx, in.Principal.ID())
		if err != nil {
			return fmt.Errorf("lock user to delete: %w", err)
		}

		if err := a.Workspaces.Handle(ctx, tx, user.ID); err != nil {
			return err
		}

		if err := a.endAccess(ctx, tx, user.ID); err != nil {
			return err
		}

		return a.forget(ctx, tx, user)
	})
	if err != nil {
		return err
	}

	if s, ok := session.From(ctx); ok {
		s.Destroy()
	}

	return nil
}

/*
endAccess ends everything that lets the user in.
*/
func (a *DeleteAccount) endAccess(ctx context.Context, tx *database.Tx, userID ids.UUID) error {
	q := models.New(tx)

	_, err := q.RevokeUserTokens(ctx, models.RevokeUserTokensParams{UserID: userID, Now: a.Clock.Now()})
	if err != nil {
		return fmt.Errorf("revoke tokens of deleted account: %w", err)
	}

	err = a.AppGrants.RevokeUserAppGrants(ctx, userID, appauth.RevokedForUserDeleted, ids.Nil())
	if err != nil {
		return fmt.Errorf("end app grants of deleted account: %w", err)
	}

	if _, err := a.Sessions.RevokeOthers(ctx, userID, ids.Nil()); err != nil {
		return fmt.Errorf("end sessions of deleted account: %w", err)
	}

	return nil
}

/*
forget removes what identifies the user, queues the removal of their
picture, tells the address they had, and announces it.
*/
func (a *DeleteAccount) forget(ctx context.Context, tx *database.Tx, user models.User) error {
	q := models.New(tx)

	if err := removeRecords(ctx, q, user.ID); err != nil {
		return err
	}

	err := q.AnonymizeUser(ctx, models.AnonymizeUserParams{
		ID: user.ID, Name: deletedName, Email: "deleted+" + user.ID.String() + "@users.invalid", Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("anonymize deleted account: %w", err)
	}

	if user.AvatarPath != nil {
		if err := a.Queue.PushTx(ctx, tx, jobs.DeleteUserAvatarArgs{Path: *user.AvatarPath}); err != nil {
			return fmt.Errorf("queue removal of deleted avatar: %w", err)
		}
	}

	err = appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.AccountDeletedMail{Name: user.Name})
	if err != nil {
		return err
	}

	return a.Events.Dispatch(ctx, tx, appevents.UserDeleted{UserID: user.ID})
}

/*
removeRecords deletes the rows that exist only for the account: second
factors, linked providers, mailed secrets and the security log.
*/
func removeRecords(ctx context.Context, q *models.Queries, userID ids.UUID) error {
	if _, err := q.DeleteUserAuthFactors(ctx, userID); err != nil {
		return fmt.Errorf("remove factors of deleted account: %w", err)
	}

	if _, err := q.DeleteUserOAuthIdentities(ctx, userID); err != nil {
		return fmt.Errorf("unlink providers of deleted account: %w", err)
	}

	if err := q.DeleteUserOneTimeTokens(ctx, &userID); err != nil {
		return fmt.Errorf("remove secrets of deleted account: %w", err)
	}

	if err := q.DeleteUserPasswordResetTokens(ctx, userID); err != nil {
		return fmt.Errorf("remove reset of deleted account: %w", err)
	}

	if err := q.DeleteUserSecurityEvents(ctx, userID); err != nil {
		return fmt.Errorf("remove security log of deleted account: %w", err)
	}

	return nil
}
