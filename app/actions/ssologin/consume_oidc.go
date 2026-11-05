/*
|--------------------------------------------------------------------------
| Consume OpenID Connect Answer
|--------------------------------------------------------------------------
|
| Reads the code an issuer sends the browser back with. It must answer
| the attempt this browser started; the code is redeemed with the
| attempt's verifier and the ID token must carry its nonce. An issuer
| cannot start a sign-in on its own here.
|
*/

package ssologin

import (
	"context"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/socialite"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
ConsumeOIDCInput is the connection, what the issuer sent back, the
browser's attempt cookie and the client.
*/
type ConsumeOIDCInput struct {
	ConnectionID ids.UUID
	Code         string
	State        string
	Denied       bool
	Browser      string
	Client       appauth.Client
}

/*
ConsumeOIDC reads an OpenID Connect answer.

Errors: [exceptions.ErrSSOFailed] for any answer that cannot sign in,
[exceptions.ErrAuthBusy] when attempts cannot be checked, or what
[CompleteSSOLogin] refuses.
*/
type ConsumeOIDC struct {
	DB       *database.DB
	Clock    clock.Clock
	Attempts appauth.SSOAttempts
	OIDC     OIDCProviders
	Complete *CompleteSSOLogin
}

/*
Handle redeems the code and acts on who it names.
*/
func (a *ConsumeOIDC) Handle(ctx context.Context, in ConsumeOIDCInput) (Result, error) {
	settle := settling{DB: a.DB, Clock: a.Clock, Complete: a.Complete}

	conn, err := findConnection(ctx, a.DB, in.ConnectionID)
	if err != nil || conn.Connection.Protocol != sso.ProtocolOIDC {
		return Result{}, exceptions.ErrSSOFailed
	}

	attempt, found, err := a.Attempts.Claim(ctx, in.State, in.Browser)
	if err != nil {
		return Result{}, busy(err)
	}

	if !found || attempt.ConnectionID != conn.Connection.ID {
		return Result{}, settle.fail(ctx, conn, "unsolicited")
	}

	if in.Denied || in.Code == "" {
		return Result{}, settle.fail(ctx, conn, "identity_provider_refused")
	}

	user, err := a.redeem(ctx, conn, in, attempt)
	if err != nil {
		return Result{}, settle.fail(ctx, conn, oidcReason(err))
	}

	identity := oidcIdentity(conn.Config, user)
	if identity.Email == "" {
		return Result{}, settle.fail(ctx, conn, "missing_email")
	}

	return settle.finish(ctx, answered{conn: conn, attempt: attempt, identity: identity, client: in.Client})
}

/*
redeem trades the code for the person it names.
*/
func (a *ConsumeOIDC) redeem(
	ctx context.Context, conn sso.Connection, in ConsumeOIDCInput, attempt appauth.SSOAttempt,
) (socialite.User, error) {
	provider, err := a.OIDC.For(ctx, conn)
	if err != nil {
		return socialite.User{}, err
	}

	token, err := provider.Exchange(ctx, in.Code, a.OIDC.attempt(conn, in.State, attempt))
	if err != nil {
		return socialite.User{}, err
	}

	return provider.User(ctx, token)
}

/*
oidcIdentity reads the person from the verified ID token: the subject,
the address, the name and the groups the connection's attribute names.
*/
func oidcIdentity(cfg sso.Config, user socialite.User) Identity {
	identity := Identity{Subject: user.ID, Email: strings.TrimSpace(user.Email), Name: user.Name}

	if cfg.AttributeMap.Groups == "" {
		return identity
	}

	if raw, ok := user.Raw[cfg.AttributeMap.Groups].([]any); ok {
		for _, group := range raw {
			if name, ok := group.(string); ok {
				identity.Groups = append(identity.Groups, name)
			}
		}
	}

	return identity
}

/*
oidcReason names why an issuer's answer was refused, for the
connection's owners.
*/
func oidcReason(err error) string {
	switch {
	case errors.Is(err, socialite.ErrInvalidIDToken):
		return "invalid_id_token"
	case errors.Is(err, socialite.ErrDenied):
		return "identity_provider_refused"
	default:
		return "exchange_failed"
	}
}
