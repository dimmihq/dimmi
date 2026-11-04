/*
|--------------------------------------------------------------------------
| Request Email Change
|--------------------------------------------------------------------------
|
| Moving an account to a new address starts with a recent proof of who is
| asking and ends with a link only the new mailbox can read. The old
| mailbox hears about the request at once, so a thief in the account is
| noticed while there is still time. Only the latest request stands: the
| link is bound to the address the account has now, so it can never move
| an account that has since moved elsewhere.
|
*/

package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How long a link to a new address works, what the secret stores it under,
and how many requests a person may make in an hour.
*/
const (
	emailChangeTTL     = 24 * time.Hour
	emailChangeDataKey = "new_email"
	emailChangesAnHour = 5
)

/*
RequestEmailChangeInput is the user, the address they want and the
client asking.
*/
type RequestEmailChangeInput struct {
	User   models.User
	Email  string
	Client appauth.Client
}

/*
EmailChange is a move to a new address: the secret's id, the address as
this answer may show it, and until when the link works or when it was
used.
*/
type EmailChange struct {
	ID          ids.UUID
	UserID      ids.UUID
	Email       string
	ExpiresAt   *time.Time
	ConfirmedAt *time.Time
}

/*
RequestEmailChange mails a link to the new address and warns the old.

Rules:
  - Five requests an hour per user.
  - The address must differ from the account's and belong to no account.
  - A new request ends the one before it.
  - The link works for 24 hours.

Errors: [exceptions.ErrEmailUnchanged], [exceptions.ErrEmailTaken],
[exceptions.ErrTooManyAttempts], or a wrapped error.
*/
type RequestEmailChange struct {
	DB          *database.DB
	Mail        *mail.Manager
	Limiter     *ratelimit.Limiter
	Links       appauth.Links
	Tokens      *appauth.OneTimeTokens
	SecurityLog *appauth.SecurityLog
}

/*
Handle starts the change and returns it, with the new address masked.
*/
func (a *RequestEmailChange) Handle(ctx context.Context, in RequestEmailChangeInput) (EmailChange, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if strings.EqualFold(in.Email, in.User.Email) {
		return EmailChange{}, exceptions.ErrEmailUnchanged
	}

	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "email-change:" + in.User.ID.String(), Limit: emailChangesAnHour, Window: time.Hour,
	})
	if err != nil {
		return EmailChange{}, err
	}

	if err := a.ensureFree(ctx, in.Email); err != nil {
		return EmailChange{}, err
	}

	var issued appauth.Issued

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		issued, err = a.Tokens.Issue(ctx, appauth.Issue{
			Purpose: enums.OneTimePurposeEmailChange, UserID: &in.User.ID, Email: in.User.Email,
			Data: map[string]any{emailChangeDataKey: in.Email}, Client: in.Client, TTL: emailChangeTTL, Live: 1,
		})
		if err != nil {
			return err
		}

		return a.tell(ctx, tx, in, issued)
	})
	if err != nil {
		return EmailChange{}, err
	}

	return EmailChange{
		ID: issued.ID, UserID: in.User.ID, Email: maskEmail(in.Email), ExpiresAt: &issued.ExpiresAt,
	}, nil
}

/*
ensureFree refuses an address another account holds.
*/
func (a *RequestEmailChange) ensureFree(ctx context.Context, email string) error {
	_, err := models.New(a.DB).FindUserByEmail(ctx, email)
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("look up new email: %w", err)
	}

	return exceptions.ErrEmailTaken
}

/*
tell mails the link to the new address, warns the old one and logs the
request.
*/
func (a *RequestEmailChange) tell(
	ctx context.Context, tx *database.Tx, in RequestEmailChangeInput, issued appauth.Issued,
) error {
	user, email := in.User, in.Email
	moved := user
	moved.Email = email

	err := appmail.ToUser(a.Mail, moved).QueueTx(ctx, tx, appmail.EmailChangeVerificationMail{
		Name: user.Name, Email: email, URL: a.Links.ConfirmEmailChange(issued.ID, issued.Token),
	})
	if err != nil {
		return err
	}

	err = appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.EmailChangeRequestedMail{
		Name: user.Name, MaskedEmail: maskEmail(email), SecurityURL: a.Links.Security(),
	})
	if err != nil {
		return err
	}

	return a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: user.ID, Type: appauth.EventEmailChangeRequested, Client: in.Client,
		Metadata: map[string]any{"new_email": maskEmail(email)},
	})
}
