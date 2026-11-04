/*
|--------------------------------------------------------------------------
| Consume Magic Link
|--------------------------------------------------------------------------
|
| Following a mailed sign-in link, or typing its code, proves the inbox.
| Only the page's script spends a link, by posting its token, so a mail
| scanner that opens the address spends nothing. A link works once; a
| code is checked against every live link of the address, and each
| wrong code counts against all of them.
|
| The account is found by address when the link is spent, not when it
| was sent: if someone registered the address in between, the inbox
| owner signs into that account and claims it from them. An address
| still without an account gets one, already verified, with no password,
| if the install still takes it: an invitation withdrawn or signup closed
| after the link went out ends what the link could do. Either way the
| inbox is now proven.
|
*/

package auth

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/session"

	"github.com/dimmihq/dimmi/app/actions/users"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
ConsumeMagicLinkInput is either the link's Token, or the Email and the
Code typed, with the client. Reauth asks for a fresh sign-in of the
session's own user.
*/
type ConsumeMagicLinkInput struct {
	Token  string
	Email  string
	Code   string
	Reauth bool
	Client appauth.Client
}

/*
ConsumeMagicLink signs in with a mailed link or code.

Rules:
  - Turned off by configuration: the grant is not allowed.
  - Ten tries a minute per address; an address blocked for guessing
    passwords across accounts is refused.
  - A link that was used, ran out or never existed: expired. A wrong
    code: invalid, counted against every live link of the address.
  - The login policy for the address is asked before the account is
    touched.
  - With Reauth on a signed-in session, the link must be for the
    session's user.
  - An unverified account is claimed from whoever registered it, unless
    this browser is signed in to it; any account is marked verified.
  - An unknown address signs up if signup is open or an invitation to
    it is still pending; otherwise the link stays unspent.

Events: [appevents.UserRegistered] for a new account.

Errors: [exceptions.ErrGrantNotAllowed], [exceptions.ErrMagicLinkExpired],
[exceptions.ErrInvalidCode], [exceptions.ErrReauthUserMismatch],
[exceptions.ErrSignupClosed], [exceptions.ErrSSORequired],
[exceptions.ErrTooManyAttempts],
[exceptions.ErrAuthBusy], those of [CompletePrimaryAuth], or a wrapped
error.
*/
type ConsumeMagicLink struct {
	DB                  *database.DB
	Clock               clock.Clock
	Events              *events.Dispatcher
	Limiter             *ratelimit.Limiter
	Lockout             *appauth.Lockout
	Tokens              *appauth.OneTimeTokens
	LoginPolicy         appauth.LoginPolicyResolver
	MagicLink           config.MagicLinkConfig
	Signup              config.SignupConfig
	SecurityLog         *appauth.SecurityLog
	CreateUser          *users.CreateUser
	ClaimAccount        *ClaimAccount
	CompletePrimaryAuth *CompletePrimaryAuth
}

/*
proof is a link or code just checked: the secret, the method it
proves, and the account at its address, nil for none yet.
*/
type proof struct {
	token   models.OneTimeToken
	method  string
	account *models.User
	client  appauth.Client
}

/*
Handle checks the link or code and signs the user in.
*/
func (a *ConsumeMagicLink) Handle(ctx context.Context, in ConsumeMagicLinkInput) (SignedIn, error) {
	if !a.MagicLink.Enabled {
		return SignedIn{}, exceptions.ErrGrantNotAllowed
	}

	if err := a.Lockout.CheckAddress(ctx, in.Client.IP); err != nil {
		return SignedIn{}, err
	}

	err := appauth.Throttle(ctx, a.Limiter, appauth.Rate{
		Key: "magic:redeem:" + in.Client.IP, Limit: 10, Window: time.Minute,
	})
	if err != nil {
		return SignedIn{}, err
	}

	p, err := a.prove(ctx, in)
	if err != nil {
		return SignedIn{}, err
	}

	if in.Reauth && !isSessionAccount(ctx, p.account) {
		return SignedIn{}, exceptions.ErrReauthUserMismatch
	}

	user, err := a.spend(ctx, p)
	if err != nil {
		return SignedIn{}, err
	}

	return a.CompletePrimaryAuth.Handle(ctx, CompletePrimaryAuthInput{
		User: user, Method: p.method, Client: in.Client, Reauth: in.Reauth,
	})
}

/*
prove checks the link or code and finds the account at its address.
*/
func (a *ConsumeMagicLink) prove(ctx context.Context, in ConsumeMagicLinkInput) (proof, error) {
	p, err := a.check(ctx, in)
	if err != nil {
		return proof{}, err
	}

	user, err := models.New(a.DB).FindUserByEmail(ctx, p.token.Email)
	if errors.Is(err, database.ErrNotFound) {
		return p, nil
	}

	if err != nil {
		return proof{}, fmt.Errorf("find user for magic link: %w", err)
	}

	p.account = &user

	return p, nil
}

/*
check finds the secret the link's token or the typed code proves,
asking the login policy for its address. Only a live secret matches a
code; a link that ran out or was used is found, and fails to be spent.
*/
func (a *ConsumeMagicLink) check(ctx context.Context, in ConsumeMagicLinkInput) (proof, error) {
	p := proof{method: appauth.MethodMagicLink, client: in.Client}

	if in.Token == "" {
		p.method = appauth.MethodEmailCode
		email := strings.ToLower(strings.TrimSpace(in.Email))

		if err := appauth.CheckLoginPolicy(ctx, a.LoginPolicy, email, p.method); err != nil {
			return proof{}, err
		}

		tok, ok, err := a.Tokens.MatchCode(ctx, enums.OneTimePurposeMagicLink, email, in.Code)
		if err != nil {
			return proof{}, err
		}

		if !ok {
			return proof{}, exceptions.ErrInvalidCode
		}

		p.token = tok

		return p, nil
	}

	tok, ok, err := a.Tokens.FindByToken(ctx, enums.OneTimePurposeMagicLink, in.Token)
	if err != nil {
		return proof{}, err
	}

	if !ok {
		return proof{}, exceptions.ErrMagicLinkExpired
	}

	p.token = tok

	return p, appauth.CheckLoginPolicy(ctx, a.LoginPolicy, tok.Email, p.method)
}

/*
spend uses the secret up and settles the account it proves, in one
transaction: created, claimed or just verified. It returns the account
as it now is.
*/
func (a *ConsumeMagicLink) spend(ctx context.Context, p proof) (models.User, error) {
	var user models.User

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		spent, err := a.Tokens.Spend(ctx, p.token)
		if err != nil {
			return err
		}

		if !spent {
			return exceptions.ErrMagicLinkExpired
		}

		user, err = a.settle(ctx, tx, p)
		if err != nil {
			return err
		}

		return a.SecurityLog.Record(ctx, appauth.Entry{
			UserID: user.ID, Type: appauth.EventMagicLinkUsed, Client: p.client,
			Metadata: map[string]any{"method": p.method},
		})
	})

	return user, err
}

/*
settle returns the account the proof signs into: a new one for an
address without any, else the existing one, claimed from whoever
registered it when the address was never proven, and verified.
*/
func (a *ConsumeMagicLink) settle(ctx context.Context, tx *database.Tx, p proof) (models.User, error) {
	if p.account == nil {
		return a.signUp(ctx, tx, p.token)
	}

	return a.ClaimAccount.Prove(ctx, tx, *p.account, p.client)
}

/*
signUp creates the verified, passwordless account a sign-up link was
sent for, named as asked or after its address, when the install still
takes the address.
*/
func (a *ConsumeMagicLink) signUp(ctx context.Context, tx *database.Tx, tok models.OneTimeToken) (models.User, error) {
	var data struct {
		Name string `json:"name"`
	}

	_ = json.Unmarshal(tok.Data, &data)

	admitted, err := admitsNewcomer(ctx, models.New(tx), a.Signup, newcomer{email: tok.Email, now: a.Clock.Now()})
	if err != nil {
		return models.User{}, err
	}

	if !admitted {
		return models.User{}, exceptions.ErrSignupClosed
	}

	user, err := a.CreateUser.Handle(ctx, users.CreateUserInput{
		Name: cmp.Or(strings.TrimSpace(data.Name), nameFromEmail(tok.Email)), Email: tok.Email, Verified: true,
		CreatedVia: appauth.CreatedViaMagicLink,
	})
	if err != nil {
		return models.User{}, err
	}

	return user, a.Events.Dispatch(ctx, tx, appevents.UserRegistered{UserID: user.ID})
}

/*
nameFromEmail names an account after the local part of its address,
"ada.lovelace@example.com" becoming "Ada Lovelace".
*/
func nameFromEmail(email string) string {
	local, _, _ := strings.Cut(email, "@")
	words := strings.FieldsFunc(local, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })

	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}

	return cmp.Or(strings.Join(words, " "), local)
}

/*
isSessionAccount reports whether the request's session is signed in as
the account, or as nobody, in which case a re-authentication is just a
sign-in. An address without an account is never the session's user.
Sign-in routes always carry a session.
*/
func isSessionAccount(ctx context.Context, account *models.User) bool {
	if account != nil {
		return isSessionUser(ctx, *account)
	}

	s, _ := session.From(ctx)
	_, signedIn := s.UserID()

	return !signedIn
}
