/*
|--------------------------------------------------------------------------
| Handle OAuth Callback
|--------------------------------------------------------------------------
|
| A person came back from a provider that vouches for an account there.
| The provider's own id for the person decides which account they get:
| an account already linked to it signs in, whatever address the
| provider now reports. Without a link, the address decides, but only
| when the provider verified it, since an unverified address proves
| nothing about who reads its mail.
|
| A verified address of an existing account links the provider to it on
| its own: the provider just proved the inbox. When that account never
| proved its address, a stranger may have registered it first, so the
| account is claimed from them exactly as a magic link would. An
| address without an account signs up, when the install takes new
| accounts or an invitation waits for it.
|
| Linking from settings and signing in afresh change nothing for anyone
| else: a fresh sign-in must come back as the session's own user, or
| nothing happens at all.
|
*/

package auth

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/users"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/providers"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
OAuthCallbackInput is the person the provider vouches for, what the
attempt was for, the user a link or fresh sign-in was started for, the
invitation token the browser carried, and the client.
*/
type OAuthCallbackInput struct {
	Identity   providers.ExternalIdentity
	Intent     string
	UserID     ids.UUID
	Invitation string
	Client     appauth.Client
}

/*
OAuthCallbackResult is how the callback ended: a sign-in, or a link
when Linked is set. InvitationMatches reports whether the carried
invitation was sent to the signed-in user's address.
*/
type OAuthCallbackResult struct {
	SignedIn          SignedIn
	Linked            bool
	InvitationMatches bool
}

/*
HandleOAuthCallback signs in, signs up or links with a provider
account.

Rules:
  - The login policy for the reported address is asked first.
  - A link attaches the account to the user it was started for.
  - An account linked before signs its user in; its details are
    refreshed.
  - Without a link, the provider must have verified the address. An
    account with that address gets the link, mailed to its owner; one
    that never proved its address is claimed and verified.
  - An unknown address signs up, verified and without a password, when
    signup is open, the carried invitation is for it, or an invitation
    to it is pending. Its picture is fetched afterwards.
  - A fresh sign-in must resolve to the user it was started for, before
    anything is written.

Events: [appevents.UserRegistered] for a new account.

Errors: [exceptions.ErrOAuthEmailUnverified],
[exceptions.ErrSignupClosed], [exceptions.ErrReauthUserMismatch],
[exceptions.ErrSSORequired], [exceptions.ErrAuthBusy], those of
[LinkOAuthIdentity], [users.CreateUser] and [CompletePrimaryAuth], or a
wrapped error.
*/
type HandleOAuthCallback struct {
	DB                  *database.DB
	Clock               clock.Clock
	Events              *events.Dispatcher
	Queue               *queue.Queue
	LoginPolicy         appauth.LoginPolicyResolver
	Signup              config.SignupConfig
	CreateUser          *users.CreateUser
	ClaimAccount        *ClaimAccount
	Link                *LinkOAuthIdentity
	CompletePrimaryAuth *CompletePrimaryAuth
}

/*
oauthMatch is the account a provider account leads to before anything
changes: the identity linked before, the account it is linked to or
the one with its address, and an identity left behind by a deleted
account.
*/
type oauthMatch struct {
	identity *models.OauthIdentity
	account  *models.User
	stale    *models.OauthIdentity
}

/*
Handle settles the account and finishes the sign-in or the link.
*/
func (a *HandleOAuthCallback) Handle(ctx context.Context, in OAuthCallbackInput) (OAuthCallbackResult, error) {
	err := appauth.CheckLoginPolicy(ctx, a.LoginPolicy, in.Identity.Email, in.Identity.Provider)
	if err != nil {
		return OAuthCallbackResult{}, err
	}

	if in.Intent == appauth.IntentLink {
		err := a.Link.Handle(ctx, LinkOAuthIdentityInput{UserID: in.UserID, Identity: in.Identity, Client: in.Client})

		return OAuthCallbackResult{Linked: true}, err
	}

	m, err := a.match(ctx, in.Identity)
	if err != nil {
		return OAuthCallbackResult{}, err
	}

	reauth := in.Intent == appauth.IntentReauth
	if reauth && (m.account == nil || m.account.ID != in.UserID) {
		return OAuthCallbackResult{}, exceptions.ErrReauthUserMismatch
	}

	user, err := a.settle(ctx, in, m)
	if err != nil {
		return OAuthCallbackResult{}, err
	}

	signedIn, err := a.CompletePrimaryAuth.Handle(ctx, CompletePrimaryAuthInput{
		User: user, Method: in.Identity.Provider, Client: in.Client, Reauth: reauth,
	})
	if err != nil {
		return OAuthCallbackResult{}, err
	}

	matches, err := a.invitationFor(ctx, in.Invitation, user.Email)

	return OAuthCallbackResult{SignedIn: signedIn, InvitationMatches: matches}, err
}

/*
match finds the account the provider account leads to.
*/
func (a *HandleOAuthCallback) match(ctx context.Context, identity providers.ExternalIdentity) (oauthMatch, error) {
	q := models.New(a.DB)
	m := oauthMatch{}

	row, err := q.FindOAuthIdentity(ctx, models.FindOAuthIdentityParams{
		Provider: identity.Provider, ProviderUserID: identity.Subject,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return oauthMatch{}, fmt.Errorf("find identity: %w", err)
	}

	if err == nil {
		user, err := q.FindUser(ctx, row.UserID)
		if err != nil {
			return oauthMatch{}, fmt.Errorf("find user of identity: %w", err)
		}

		if !user.IsDeleted() {
			return oauthMatch{identity: &row, account: &user}, nil
		}

		m.stale = &row
	}

	if !identity.EmailVerified {
		return oauthMatch{}, exceptions.ErrOAuthEmailUnverified
	}

	user, err := q.FindUserByEmail(ctx, identity.Email)
	if errors.Is(err, database.ErrNotFound) {
		return m, nil
	}

	if err != nil {
		return oauthMatch{}, fmt.Errorf("find user by provider email: %w", err)
	}

	m.account = &user

	return m, nil
}

/*
settle writes what the match decided, in one transaction, and returns
the account as it now is.
*/
func (a *HandleOAuthCallback) settle(ctx context.Context, in OAuthCallbackInput, m oauthMatch) (models.User, error) {
	var user models.User

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		now := a.Clock.Now()

		if m.stale != nil {
			_, err := models.New(tx).DeleteOAuthIdentity(ctx, models.DeleteOAuthIdentityParams{
				ID: m.stale.ID, UserID: m.stale.UserID,
			})
			if err != nil {
				return fmt.Errorf("remove identity of deleted user: %w", err)
			}
		}

		var err error

		switch {
		case m.identity != nil:
			user = *m.account
			err = a.Link.Touch(ctx, m.identity.ID, in.Identity, &now)
		case m.account != nil:
			user, err = a.adopt(ctx, tx, *m.account, in)
		default:
			user, err = a.signUp(ctx, tx, in)
		}

		return err
	})

	return user, err
}

/*
adopt links the provider account to the account with its verified
address, proving that address first.
*/
func (a *HandleOAuthCallback) adopt(
	ctx context.Context, tx *database.Tx, user models.User, in OAuthCallbackInput,
) (models.User, error) {
	user, err := a.ClaimAccount.Prove(ctx, tx, user, in.Client)
	if err != nil {
		return models.User{}, err
	}

	return user, a.Link.Attach(ctx, tx, user, LinkOAuthIdentityInput{
		UserID: user.ID, Identity: in.Identity, Client: in.Client,
	})
}

/*
signUp creates the verified, passwordless account of an unknown
address, linked to the provider account, when the install lets it.
*/
func (a *HandleOAuthCallback) signUp(ctx context.Context, tx *database.Tx, in OAuthCallbackInput) (models.User, error) {
	identity := in.Identity
	if err := a.admit(ctx, in); err != nil {
		return models.User{}, err
	}

	user, err := a.CreateUser.Handle(ctx, users.CreateUserInput{
		Name: cmp.Or(strings.TrimSpace(identity.Name), nameFromEmail(identity.Email)), Email: identity.Email,
		Verified: true, CreatedVia: identity.Provider,
	})
	if err != nil {
		return models.User{}, err
	}

	now := a.Clock.Now()
	if err := a.Link.Create(ctx, user.ID, identity, &now); err != nil {
		return models.User{}, err
	}

	if err := a.Events.Dispatch(ctx, tx, appevents.UserRegistered{UserID: user.ID}); err != nil {
		return models.User{}, err
	}

	if identity.AvatarURL == "" {
		return user, nil
	}

	err = a.Queue.PushTx(ctx, tx, jobs.FetchUserAvatarArgs{UserID: user.ID, URL: identity.AvatarURL})
	if err != nil {
		return models.User{}, fmt.Errorf("queue avatar fetch: %w", err)
	}

	return user, nil
}

/*
admit decides whether the address may sign up: always while signup is
open, else only with an invitation to it, carried or pending.
*/
func (a *HandleOAuthCallback) admit(ctx context.Context, in OAuthCallbackInput) error {
	if a.Signup.Mode == config.SignupOpen {
		return nil
	}

	carried, err := a.invitationFor(ctx, in.Invitation, in.Identity.Email)
	if err != nil || carried {
		return err
	}

	pending, err := models.New(a.DB).CountPendingInvitationsForEmail(ctx, models.CountPendingInvitationsForEmailParams{
		Email: in.Identity.Email, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("count pending invitations: %w", err)
	}

	if pending == 0 {
		return exceptions.ErrSignupClosed
	}

	return nil
}

/*
invitationFor reports whether the invitation token belongs to a pending
invitation to the address that has not expired. No token is no match.
*/
func (a *HandleOAuthCallback) invitationFor(ctx context.Context, token, email string) (bool, error) {
	if token == "" {
		return false, nil
	}

	inv, err := models.New(a.DB).FindInvitationByToken(ctx, appauth.Digest(token))
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("find carried invitation: %w", err)
	}

	pending := enums.InvitationStatus(inv.Status) == enums.InvitationStatusPending && inv.ExpiresAt.After(a.Clock.Now())

	return pending && strings.EqualFold(inv.Email, email), nil
}
