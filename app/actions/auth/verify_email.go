/*
|--------------------------------------------------------------------------
| Verify Email
|--------------------------------------------------------------------------
|
| Opening a verification link proves the account's address, but it does
| not by itself say that whoever registered the account owns that inbox:
| a stranger may have signed up with someone else's address, and the
| mail then reaches the real owner, who may click it out of habit. A
| click must never hand that person's address to the stranger.
|
| So only a browser signed in to the account verifies it outright: the
| registrant is holding the session and the inbox together. Any other
| browser, such as a phone or a mail scanner that runs the page, spends
| nothing and is answered that a choice is needed. The person may sign
| in here and open the link again, or say they never created the account,
| which claims it from whoever did and leaves them nothing.
|
| A wrong token, an unknown id and an id of another kind of secret fail
| alike, so a guess learns nothing. Once the account is verified, by
| this link or any other way, every link answers that it is, even a
| spent or expired one, so a second click never shows an error. A link
| sent to an address the account no longer has is stale, never valid.
|
*/

package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
VerifyEmailInput is the link's public id and token, and the client
that opened it. Claim says the person holding the link did not create
the account and takes it back.
*/
type VerifyEmailInput struct {
	ID     string
	Token  string
	Claim  bool
	Client appauth.Client
}

/*
Verification is the outcome: whose address was verified, which one, and
when it first was.
*/
type Verification struct {
	ID         ids.UUID
	UserID     ids.UUID
	Email      string
	VerifiedAt time.Time
}

/*
VerifyEmail proves the address of the account a link names.

Rules:
  - Ten tries a minute per address; five wrong tokens burn the link.
  - Unknown id, wrong kind of secret, wrong token, or a deleted account:
    invalid, all alike.
  - A link sent to an address the account no longer has: stale.
  - An account already verified: verified, nothing changes.
  - Otherwise an expired or spent link: expired.
  - A browser not signed in to the account, without Claim: a choice is
    needed, and the link stays good.
  - With Claim, the account is claimed by [ClaimAccount] as it is
    verified.

Events: [appevents.UserEmailVerified], and those of [ClaimAccount].

Errors: [exceptions.ErrInvalidVerificationToken],
[exceptions.ErrVerificationLinkStale],
[exceptions.ErrVerificationLinkExpired],
[exceptions.ErrVerificationNeedsChoice],
[exceptions.ErrTooManyAttempts], [exceptions.ErrAuthBusy], or a
wrapped error.
*/
type VerifyEmail struct {
	DB      *database.DB
	Clock   clock.Clock
	Events  *events.Dispatcher
	IDs     *ids.Registry
	Limiter *ratelimit.Limiter
	Tokens  *appauth.OneTimeTokens
	Claim   *ClaimAccount
}

/*
Handle verifies the address.
*/
func (a *VerifyEmail) Handle(ctx context.Context, in VerifyEmailInput) (Verification, error) {
	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "verification:redeem:" + in.Client.IP, Limit: 10, Window: time.Minute,
	})
	if err != nil {
		return Verification{}, err
	}

	tok, user, err := a.find(ctx, in)
	if err != nil {
		return Verification{}, err
	}

	if !strings.EqualFold(tok.Email, user.Email) {
		return Verification{}, exceptions.ErrVerificationLinkStale
	}

	if user.IsVerified() {
		return Verification{ID: tok.ID, UserID: user.ID, Email: user.Email, VerifiedAt: *user.EmailVerifiedAt}, nil
	}

	if !in.Claim && !signedInAs(ctx, user) {
		return Verification{}, a.needsChoice(tok)
	}

	return a.verify(ctx, tok, user, in.Client)
}

/*
needsChoice answers a browser that may not verify the account as it
stands: a link still good waits for the person's choice, and one that
ran out or was spent says so at once, since no choice could save it.
*/
func (a *VerifyEmail) needsChoice(tok models.OneTimeToken) error {
	if tok.ConsumedAt != nil || !tok.ExpiresAt.After(a.Clock.Now()) {
		return exceptions.ErrVerificationLinkExpired
	}

	return exceptions.ErrVerificationNeedsChoice
}

/*
find returns the link and its account when the token proves the link,
counting a wrong token against it.
*/
func (a *VerifyEmail) find(ctx context.Context, in VerifyEmailInput) (models.OneTimeToken, models.User, error) {
	id, err := a.IDs.Decode(string(models.PrefixOneTimeToken), in.ID)
	if err != nil {
		return models.OneTimeToken{}, models.User{}, exceptions.ErrInvalidVerificationToken
	}

	tok, ok, err := a.Tokens.Find(ctx, enums.OneTimePurposeEmailVerification, id)
	if err != nil {
		return models.OneTimeToken{}, models.User{}, err
	}

	if !ok {
		return models.OneTimeToken{}, models.User{}, exceptions.ErrInvalidVerificationToken
	}

	if !a.Tokens.Proves(tok, in.Token) || tok.Attempts >= appauth.MaxSecretAttempts {
		if err := a.Tokens.Miss(ctx, tok); err != nil {
			return models.OneTimeToken{}, models.User{}, err
		}

		return models.OneTimeToken{}, models.User{}, exceptions.ErrInvalidVerificationToken
	}

	user, err := models.New(a.DB).FindUser(ctx, *tok.UserID)
	if err != nil {
		return models.OneTimeToken{}, models.User{}, fmt.Errorf("find user to verify: %w", err)
	}

	if user.IsDeleted() {
		return models.OneTimeToken{}, models.User{}, exceptions.ErrInvalidVerificationToken
	}

	return tok, user, nil
}

/*
verify spends the link and marks the address proven, in one
transaction, claiming the account unless this browser is signed in to
it. A link that ran out or was spent cannot be spent again.
*/
func (a *VerifyEmail) verify(
	ctx context.Context, tok models.OneTimeToken, user models.User, client appauth.Client,
) (Verification, error) {
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		spent, err := a.Tokens.Spend(ctx, tok)
		if err != nil {
			return err
		}

		if !spent {
			return exceptions.ErrVerificationLinkExpired
		}

		user, err = a.Claim.Prove(ctx, tx, user, client)
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.UserEmailVerified{UserID: user.ID})
	})
	if err != nil {
		return Verification{}, err
	}

	return Verification{ID: tok.ID, UserID: user.ID, Email: user.Email, VerifiedAt: *user.EmailVerifiedAt}, nil
}
