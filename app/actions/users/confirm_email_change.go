/*
|--------------------------------------------------------------------------
| Confirm Email Change
|--------------------------------------------------------------------------
|
| Opening the link in the new mailbox finishes the move, from any device,
| signed in or not: the token is the proof. Everything that cannot be
| used fails alike and takes the same work: an unknown id, a wrong token,
| one spent or expired. Five wrong tokens burn the link.
|
| Secrets mailed to the old address end with the move, reset links
| included: whoever keeps that mailbox must not be able to come back in
| through it. Linked providers and app grants stay, since neither depends
| on the address. Pending invitations stay with the address they were
| sent to.
|
*/

package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/session"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConfirmEmailChangeInput is the link's public id and token, and the
client that opened it.
*/
type ConfirmEmailChangeInput struct {
	ID     string
	Token  string
	Client appauth.Client
}

/*
ConfirmEmailChange moves the account to the address the link was sent to.

Rules:
  - Ten tries a minute per address; five wrong tokens burn the link.
  - Unknown, wrong, spent or expired: invalid, all alike.
  - An account that left the address it had at the request: invalid.
  - An address another account took meanwhile: the change is dropped.
  - A browser signed in to the account moves to a new session id.

Events: [appevents.UserUpdated].

Errors: [exceptions.ErrInvalidEmailChangeToken],
[exceptions.ErrEmailChangeTaken], [exceptions.ErrTooManyAttempts], or a
wrapped error.
*/
type ConfirmEmailChange struct {
	DB          *database.DB
	Clock       clock.Clock
	Events      *events.Dispatcher
	Mail        *mail.Manager
	Limiter     *ratelimit.Limiter
	IDs         *ids.Registry
	Links       appauth.Links
	Tokens      *appauth.OneTimeTokens
	SecurityLog *appauth.SecurityLog
}

/*
Handle finishes the change and returns it with the new address.
*/
func (a *ConfirmEmailChange) Handle(ctx context.Context, in ConfirmEmailChangeInput) (EmailChange, error) {
	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "email-change:redeem:" + in.Client.IP, Limit: 10, Window: time.Minute,
	})
	if err != nil {
		return EmailChange{}, err
	}

	tok, email, err := a.find(ctx, in)
	if err != nil {
		return EmailChange{}, err
	}

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return a.move(ctx, tx, moveInput{tok: tok, email: email, client: in.Client})
	})
	if errors.Is(err, exceptions.ErrEmailChangeTaken) {
		if _, spendErr := a.Tokens.Spend(ctx, tok); spendErr != nil {
			return EmailChange{}, spendErr
		}
	}

	if err != nil {
		return EmailChange{}, err
	}

	if s, ok := session.From(ctx); ok {
		if current, signedIn := s.UserID(); signedIn && current == *tok.UserID {
			s.Regenerate()
		}
	}

	now := a.Clock.Now()

	return EmailChange{ID: tok.ID, UserID: *tok.UserID, Email: email, ConfirmedAt: &now}, nil
}

/*
find returns the link and the address it moves to when the token proves
the link, counting a wrong token against it.

SECURITY: An unknown id is compared against a digest of nothing, so it
takes the same work as a wrong token for a real link.
*/
func (a *ConfirmEmailChange) find(ctx context.Context, in ConfirmEmailChangeInput) (
	models.OneTimeToken, string, error,
) {
	id, err := a.IDs.Decode(string(models.PrefixOneTimeToken), in.ID)
	if err != nil {
		id = ids.Nil()
	}

	tok, known, err := a.Tokens.Find(ctx, enums.OneTimePurposeEmailChange, id)
	if err != nil {
		return models.OneTimeToken{}, "", err
	}

	proves := hashing.Equal(appauth.Digest(in.Token), tok.TokenHash)
	if !known {
		return models.OneTimeToken{}, "", exceptions.ErrInvalidEmailChangeToken
	}

	if !proves || tok.Attempts >= appauth.MaxSecretAttempts {
		if err := a.Tokens.Miss(ctx, tok); err != nil {
			return models.OneTimeToken{}, "", err
		}

		return models.OneTimeToken{}, "", exceptions.ErrInvalidEmailChangeToken
	}

	var data map[string]string
	if err := json.Unmarshal(tok.Data, &data); err != nil {
		return models.OneTimeToken{}, "", fmt.Errorf("read email change: %w", err)
	}

	return tok, data[emailChangeDataKey], nil
}

/*
moveInput is a proven link, where it moves the account, and who opened
it.
*/
type moveInput struct {
	tok    models.OneTimeToken
	email  string
	client appauth.Client
}

/*
move spends the link and moves the account, in one transaction. A link
that ran out or was spent cannot be spent again.
*/
func (a *ConfirmEmailChange) move(ctx context.Context, tx *database.Tx, in moveInput) error {
	spent, err := a.Tokens.Spend(ctx, in.tok)
	if err != nil {
		return err
	}

	if !spent {
		return exceptions.ErrInvalidEmailChangeToken
	}

	q := models.New(tx)

	user, err := q.LockUser(ctx, *in.tok.UserID)
	if err != nil {
		return fmt.Errorf("lock user to move: %w", err)
	}

	if user.IsDeleted() || !strings.EqualFold(user.Email, in.tok.Email) {
		return exceptions.ErrInvalidEmailChangeToken
	}

	now := a.Clock.Now()

	_, err = q.UpdateUserEmail(ctx, models.UpdateUserEmailParams{ID: user.ID, Email: in.email, Now: now})
	if database.IsUniqueViolation(err, "users_email_key") {
		return exceptions.ErrEmailChangeTaken
	}

	if err != nil {
		return fmt.Errorf("move account to new email: %w", err)
	}

	if err := a.endOldSecrets(ctx, q, user.ID); err != nil {
		return err
	}

	return a.announce(ctx, tx, user, in)
}

/*
endOldSecrets ends the reset link and every other secret mailed to the
address the account left.
*/
func (a *ConfirmEmailChange) endOldSecrets(ctx context.Context, q *models.Queries, userID ids.UUID) error {
	if err := q.DeleteUserPasswordResetTokens(ctx, userID); err != nil {
		return fmt.Errorf("end reset link of old email: %w", err)
	}

	err := q.ExpireUserOneTimeTokens(ctx, models.ExpireUserOneTimeTokensParams{UserID: &userID, Now: a.Clock.Now()})
	if err != nil {
		return fmt.Errorf("end secrets of old email: %w", err)
	}

	return nil
}

/*
announce tells the old address, logs the move and lets teammates know.
*/
func (a *ConfirmEmailChange) announce(ctx context.Context, tx *database.Tx, old models.User, in moveInput) error {
	err := appmail.ToUser(a.Mail, old).QueueTx(ctx, tx, appmail.EmailChangedMail{
		Name: old.Name, MaskedEmail: maskEmail(in.email), SecurityURL: a.Links.Security(),
	})
	if err != nil {
		return err
	}

	err = a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: old.ID, Type: appauth.EventEmailChanged, Client: in.client,
		Metadata: map[string]any{"old_email": maskEmail(old.Email), "new_email": maskEmail(in.email)},
	})
	if err != nil {
		return err
	}

	memberships, err := membershipsOf(ctx, models.New(tx), old.ID)
	if err != nil {
		return err
	}

	return a.Events.Dispatch(ctx, tx, appevents.UserUpdated{
		UserID: old.ID, Changes: []string{"email"}, Memberships: memberships,
	})
}
