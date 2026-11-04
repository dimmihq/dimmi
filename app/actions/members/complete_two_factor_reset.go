/*
|--------------------------------------------------------------------------
| Complete Two-Factor Reset
|--------------------------------------------------------------------------
|
| The teammate opens the link an admin had mailed them and turns their
| second factor off, from any device, signed in or not. The link proves
| the mailbox; an account with a password must prove that too, so a
| stolen mailbox alone never weakens it. Five wrong tokens or passwords
| burn the link. Everything that cannot be used fails alike.
|
| Turning the factor off ends every session and first-party app of the
| account, lifts the lock failed guesses set on it, and tells the person.
| They then sign in and meet the workspace's requirement, if it has one,
| by setting up a new factor.
|
*/

package members

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/factors"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CompleteTwoFactorResetInput is the link's public id and token, the
account's password when it has one, and the client that opened it.
*/
type CompleteTwoFactorResetInput struct {
	ID       string
	Token    string
	Password string
	Client   appauth.Client
}

/*
CompleteTwoFactorReset turns the second factor off with a mailed link.

Rules:
  - Ten tries a minute per address; five wrong tokens or passwords burn
    the link.
  - Unknown, wrong, spent or expired: invalid, all alike.
  - An account with a password needs it; without one, the link alone.
  - Every factor goes, even where a workspace requires one.
  - Every session and first-party app grant of the account ends.
  - The sign-in lock of the account is lifted.

Events: [appevents.SessionRevoked] for each session that ended, and
[appevents.MemberTwoFactorResetCompleted].

Errors: [exceptions.ErrInvalidTwoFactorResetToken],
[exceptions.ErrResetPasswordRequired], [exceptions.ErrInvalidCredentials],
[exceptions.ErrAuthBusy], [exceptions.ErrTooManyAttempts], or a wrapped
error.
*/
type CompleteTwoFactorReset struct {
	DB          *database.DB
	Clock       clock.Clock
	Events      *events.Dispatcher
	Mail        *mail.Manager
	Hash        *hashing.Manager
	Limiter     *ratelimit.Limiter
	IDs         *ids.Registry
	Links       appauth.Links
	Tokens      *appauth.OneTimeTokens
	SecurityLog *appauth.SecurityLog
	Lockout     *appauth.Lockout
	Sessions    *session.Manager
	AppGrants   appauth.AppGrants
	RemoveAll   *factors.RemoveAllAuthFactors
}

/*
proven is a link whose token and password checked out: the secret, the
account, and the workspace and member it was sent from.
*/
type proven struct {
	tok         models.OneTimeToken
	user        models.User
	workspaceID ids.UUID
	memberID    ids.UUID
}

/*
Handle turns the factor off and returns the link, now used.
*/
func (a *CompleteTwoFactorReset) Handle(
	ctx context.Context, in CompleteTwoFactorResetInput,
) (TwoFactorReset, error) {
	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: twoFactorResetRateScope + "redeem:" + in.Client.IP, Limit: 10, Window: time.Minute,
	})
	if err != nil {
		return TwoFactorReset{}, err
	}

	link, err := a.find(ctx, in)
	if err != nil {
		return TwoFactorReset{}, err
	}

	if err := a.prove(ctx, link, in.Password); err != nil {
		return TwoFactorReset{}, err
	}

	scoped := tenancy.With(ctx, link.workspaceID.String())

	err = a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return a.reset(ctx, tx, link, in.Client)
	})
	if err != nil {
		return TwoFactorReset{}, err
	}

	if err := a.Lockout.Clear(ctx, link.user.Email); err != nil {
		return TwoFactorReset{}, err
	}

	return TwoFactorReset{ID: link.tok.ID, MemberID: link.memberID, ExpiresAt: link.tok.ExpiresAt}, nil
}

/*
find returns the link when the token proves it, counting a wrong token
against it.

SECURITY: An unknown id is compared against a digest of nothing, so it
takes the same work as a wrong token for a real link.
*/
func (a *CompleteTwoFactorReset) find(ctx context.Context, in CompleteTwoFactorResetInput) (proven, error) {
	id, err := a.IDs.Decode(string(models.PrefixOneTimeToken), in.ID)
	if err != nil {
		id = ids.Nil()
	}

	tok, known, err := a.Tokens.Find(ctx, enums.OneTimePurposeTwoFactorReset, id)
	if err != nil {
		return proven{}, err
	}

	proves := a.Tokens.Proves(tok, in.Token)
	if !known {
		return proven{}, exceptions.ErrInvalidTwoFactorResetToken
	}

	if !proves || tok.Attempts >= appauth.MaxSecretAttempts || tok.ConsumedAt != nil ||
		!tok.ExpiresAt.After(a.Clock.Now()) {
		return proven{}, a.miss(ctx, tok, exceptions.ErrInvalidTwoFactorResetToken)
	}

	return a.read(ctx, tok)
}

/*
read loads the account and the workspace the link was sent from.
*/
func (a *CompleteTwoFactorReset) read(ctx context.Context, tok models.OneTimeToken) (proven, error) {
	var data map[string]string
	if err := json.Unmarshal(tok.Data, &data); err != nil {
		return proven{}, fmt.Errorf("read two-factor reset: %w", err)
	}

	user, err := models.New(a.DB).FindUser(ctx, *tok.UserID)
	if err != nil {
		return proven{}, fmt.Errorf("find user to reset: %w", err)
	}

	if user.IsDeleted() {
		return proven{}, exceptions.ErrInvalidTwoFactorResetToken
	}

	workspaceID, wsErr := ids.Parse(data[resetDataWorkspaceKey])
	memberID, memberErr := ids.Parse(data[resetDataMemberKey])

	if err := errors.Join(wsErr, memberErr); err != nil {
		return proven{}, fmt.Errorf("read two-factor reset workspace: %w", err)
	}

	return proven{tok: tok, user: user, workspaceID: workspaceID, memberID: memberID}, nil
}

/*
prove checks the account's password when it has one. A wrong one counts
against the link like a wrong token.
*/
func (a *CompleteTwoFactorReset) prove(ctx context.Context, link proven, password string) error {
	if !link.user.HasPassword() {
		return nil
	}

	if password == "" {
		return exceptions.ErrResetPasswordRequired
	}

	ok, err := a.Hash.Check(ctx, password, *link.user.PasswordHash)
	if errors.Is(err, hashing.ErrBusy) {
		return exceptions.ErrAuthBusy
	}

	if err != nil && !errors.Is(err, hashing.ErrInvalidHash) {
		return fmt.Errorf("check password for two-factor reset: %w", err)
	}

	if ok {
		return nil
	}

	return a.miss(ctx, link.tok, exceptions.ErrInvalidCredentials.OnField("password"))
}

/*
miss counts a wrong guess at the link and returns the refusal.
*/
func (a *CompleteTwoFactorReset) miss(ctx context.Context, tok models.OneTimeToken, refusal error) error {
	if err := a.Tokens.Miss(ctx, tok); err != nil {
		return err
	}

	return refusal
}

/*
reset spends the link and turns the factor off, signing the account out
everywhere, in one transaction.
*/
func (a *CompleteTwoFactorReset) reset(
	ctx context.Context, tx *database.Tx, link proven, client appauth.Client,
) error {
	spent, err := a.Tokens.Spend(ctx, link.tok)
	if err != nil {
		return err
	}

	if !spent {
		return exceptions.ErrInvalidTwoFactorResetToken
	}

	userID := link.user.ID

	_, err = a.RemoveAll.Handle(ctx, factors.RemoveAllAuthFactorsInput{
		UserID: userID, Client: client, KeepGrant: ids.Nil(), Force: true,
	})
	if err != nil {
		return err
	}

	if err := a.signOut(ctx, tx, userID); err != nil {
		return err
	}

	return a.announce(ctx, tx, link, client)
}

/*
signOut ends every session and first-party app grant of the account,
announcing each session.
*/
func (a *CompleteTwoFactorReset) signOut(ctx context.Context, tx *database.Tx, userID ids.UUID) error {
	err := a.AppGrants.RevokeUserAppGrants(ctx, userID, appauth.RevokedForTwoFactorDisabled, ids.Nil())
	if err != nil {
		return fmt.Errorf("end app grants after two-factor reset: %w", err)
	}

	live, err := a.Sessions.Sessions(ctx, userID)
	if err != nil {
		return fmt.Errorf("list sessions to end: %w", err)
	}

	if _, err := a.Sessions.RevokeOthers(ctx, userID, ids.Nil()); err != nil {
		return fmt.Errorf("end sessions after two-factor reset: %w", err)
	}

	for _, s := range live {
		if err := a.Events.Dispatch(ctx, tx, appevents.SessionRevoked{UserID: userID, SessionID: s.ID}); err != nil {
			return err
		}
	}

	return nil
}

/*
announce logs the reset on the account, tells the person and records it
in the workspace the link came from.
*/
func (a *CompleteTwoFactorReset) announce(
	ctx context.Context, tx *database.Tx, link proven, client appauth.Client,
) error {
	err := a.SecurityLog.Record(ctx, appauth.Entry{
		UserID: link.user.ID, Type: appauth.EventTwoFactorResetDone, Client: client,
	})
	if err != nil {
		return err
	}

	err = appmail.ToUser(a.Mail, link.user).QueueTx(ctx, tx, appmail.TwoFactorResetCompletedMail{
		Name: link.user.Name, SecurityURL: a.Links.Security(),
	})
	if err != nil {
		return err
	}

	return a.Events.Dispatch(ctx, tx, appevents.MemberTwoFactorResetCompleted{
		WorkspaceID: link.workspaceID, MemberID: link.memberID, UserID: link.user.ID,
	})
}
