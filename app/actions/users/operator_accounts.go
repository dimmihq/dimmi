/*
|--------------------------------------------------------------------------
| Operator Accounts
|--------------------------------------------------------------------------
|
| What the operator of a self-hosted server does for its people from the
| command line, when the app cannot help: hand over a password reset
| link without mail, set a password, vouch for an email, and sign
| someone out everywhere. Each change ends the same sessions and leaves
| the same records as the matching change in the app, so a stolen
| session never survives an operator's rescue.
|
*/

package users

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
operatorResetTTL is how long a reset link the operator hands over works.
*/
const operatorResetTTL = time.Hour

/*
OperatorAccounts holds the operator's rescues of one account. Every
change runs in one transaction with its records.
*/
type OperatorAccounts struct {
	DB                *database.DB
	Clock             clock.Clock
	Events            *events.Dispatcher
	Hash              *hashing.Manager
	Sessions          *session.Manager
	AppGrants         appauth.AppGrants
	Links             appauth.Links
	MinPasswordLength int
}

/*
ResetLink replaces the user's password reset with a fresh one and
returns its link, which works for 60 minutes.
*/
func (a *OperatorAccounts) ResetLink(ctx context.Context, user models.User) (string, error) {
	now := a.Clock.Now()
	plain, digest := appauth.NewSecret()

	reset, err := models.New(a.DB).ReplacePasswordResetToken(ctx, models.ReplacePasswordResetTokenParams{
		ID: ids.NewV7At(now), UserID: user.ID, TokenHash: digest, ExpiresAt: now.Add(operatorResetTTL), Now: now,
	})
	if err != nil {
		return "", fmt.Errorf("store password reset: %w", err)
	}

	return a.Links.ResetPassword(reset.ID, plain), nil
}

/*
SetPassword gives the user a new password and signs them out of every
session and first-party app.

Errors: [exceptions.ErrPasswordTooShort], or a wrapped error.
*/
func (a *OperatorAccounts) SetPassword(ctx context.Context, user models.User, password string) error {
	if utf8.RuneCountInString(password) < a.MinPasswordLength {
		return exceptions.ErrPasswordTooShort.With("min", a.MinPasswordLength)
	}

	hash, err := a.Hash.Make(ctx, password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		err := models.New(tx).UpdateUserPassword(ctx, models.UpdateUserPasswordParams{
			ID: user.ID, PasswordHash: &hash, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("store password: %w", err)
		}

		return a.endEverywhere(ctx, tx, user, appauth.RevokedForPasswordReset)
	})
}

/*
Verify marks the user's email as proven.
*/
func (a *OperatorAccounts) Verify(ctx context.Context, user models.User) error {
	err := models.New(a.DB).MarkUserEmailVerified(ctx, models.MarkUserEmailVerifiedParams{
		ID: user.ID, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}

	return nil
}

/*
Join makes the user a member of the workspace with the slug, with the
role, as an operator may do for any role, owner included.

Errors: [exceptions.ErrWorkspaceNotFound] for an unknown or deleted
workspace, [exceptions.ErrAlreadyMember], or a wrapped error.
*/
func (a *OperatorAccounts) Join(
	ctx context.Context, user models.User, slug string, role enums.Role,
) (models.WorkspaceMember, error) {
	w, err := models.New(a.DB).FindWorkspaceBySlug(ctx, slug)
	if errors.Is(err, database.ErrNotFound) || (err == nil && w.DeletedAt != nil) {
		return models.WorkspaceMember{}, exceptions.ErrWorkspaceNotFound
	}

	if err != nil {
		return models.WorkspaceMember{}, fmt.Errorf("find workspace: %w", err)
	}

	var member models.WorkspaceMember

	err = a.DB.Transaction(tenancy.With(ctx, w.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		via := "cli"

		member, err = models.New(tx).CreateWorkspaceMember(ctx, models.CreateWorkspaceMemberParams{
			WorkspaceID: w.ID, UserID: user.ID, Role: string(role), JoinedVia: &via, Now: a.Clock.Now(),
		})
		if database.IsUniqueViolation(err, "") {
			return exceptions.ErrAlreadyMember
		}

		if err != nil {
			return fmt.Errorf("add member: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.MemberJoined{
			WorkspaceID: w.ID, MemberID: member.ID, UserID: user.ID, JoinedVia: via,
		})
	})

	return member, err
}

/*
SignOut ends every browser session and first-party app grant of the
user. Access tokens keep working: they belong to scripts, not people.
*/
func (a *OperatorAccounts) SignOut(ctx context.Context, user models.User) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return a.endEverywhere(ctx, tx, user, appauth.RevokedByOperator)
	})
}

/*
endEverywhere ends the user's sessions and app grants and announces it, so
their open connections close.
*/
func (a *OperatorAccounts) endEverywhere(ctx context.Context, tx *database.Tx, user models.User, reason string) error {
	if _, err := a.Sessions.RevokeOthers(ctx, user.ID, ids.Nil()); err != nil {
		return fmt.Errorf("end sessions: %w", err)
	}

	if err := a.AppGrants.RevokeUserAppGrants(ctx, user.ID, reason, ids.Nil()); err != nil {
		return fmt.Errorf("end app grants: %w", err)
	}

	return a.Events.Dispatch(ctx, tx, appevents.UserSignedOutEverywhere{UserID: user.ID})
}
