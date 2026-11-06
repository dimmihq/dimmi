/*
|--------------------------------------------------------------------------
| OpenID Connect Provider Of A Connection
|--------------------------------------------------------------------------
|
| An OpenID Connect connection signs people in through its issuer with
| the client the company registered there. Every attempt carries its own
| state, PKCE verifier and nonce, kept with the attempt, so a code can
| only be redeemed by the browser that asked for it. The issuer is
| reached with the guard that keeps requests off private networks.
|
*/

package ssologin

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/actions/sso"
	appauth "github.com/dimmihq/dimmi/app/auth"
)

/*
OIDCProviders builds the OpenID Connect side of a connection.
*/
type OIDCProviders struct {
	Keys   encryption.KeyManager
	Links  appauth.Links
	Social *socialite.Manager
}

/*
For returns the provider of the connection.
*/
func (p OIDCProviders) For(ctx context.Context, conn sso.Connection) (socialite.Provider, error) {
	secret, err := sso.OpenSecret(ctx, p.Keys, conn.Connection)
	if err != nil {
		return nil, err
	}

	scopes := make([]any, 0, len(conn.Config.Scopes))
	for _, scope := range conn.Config.Scopes {
		scopes = append(scopes, scope)
	}

	provider, err := p.Social.Build(appauth.MethodSSO, config.Section{
		"driver": "oidc", "issuer": conn.Config.Issuer, "client_id": conn.Config.ClientID,
		"client_secret": secret.ClientSecret, "scopes": scopes,
	})
	if err != nil {
		return nil, fmt.Errorf("build openid provider: %w", err)
	}

	return provider, nil
}

/*
AuthURL returns the issuer's sign-in address for the attempt, giving it
a fresh verifier and nonce.
*/
func (p OIDCProviders) AuthURL(
	ctx context.Context, conn sso.Connection, relay string, attempt *appauth.SSOAttempt,
) (string, error) {
	provider, err := p.For(ctx, conn)
	if err != nil {
		return "", err
	}

	attempt.Verifier, attempt.Nonce = str.Random(64), str.Random(32)

	address, err := provider.AuthURL(ctx, p.attempt(conn, relay, *attempt))
	if err != nil {
		return "", fmt.Errorf("build openid request: %w", err)
	}

	return address, nil
}

/*
attempt is the socialite view of a stored attempt.
*/
func (p OIDCProviders) attempt(conn sso.Connection, relay string, a appauth.SSOAttempt) socialite.Attempt {
	return socialite.Attempt{
		State: relay, Verifier: a.Verifier, Nonce: a.Nonce, RedirectURL: p.Links.SSOCallback(conn.Connection.ID),
	}
}
