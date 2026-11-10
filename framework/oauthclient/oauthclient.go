/*
|--------------------------------------------------------------------------
| Account Connections
|--------------------------------------------------------------------------
|
| A tenant connects an account it holds at another service, so the
| application can act there: post messages, read records, file issues.
| Every such service speaks the OAuth 2 authorization code flow, so a
| driver only says where to send the browser, how to trade the returned
| code for tokens, how to renew them and how to give them back. Whatever
| the service, the application gets the same grant back.
|
| This is not signing in: the person is already signed in here, and the
| tokens act for their tenant at the other service. Each attempt carries
| its own state and PKCE verifier and is bound to the browser session
| that started it, so a callback link opened anywhere else is worthless.
|
*/

package oauthclient

import (
	"context"
	"time"
)

/*
Token is one credential a provider issued. Value is the secret itself
and Expiry when it stops working, zero when the provider gave none.
*/
type Token struct {
	Value  string
	Expiry time.Time
}

/*
Expired reports whether the token stops working within the skew of now.
A token without an expiry never does.
*/
func (t *Token) Expired(now time.Time, skew time.Duration) bool {
	return t != nil && !t.Expiry.IsZero() && !now.Add(skew).Before(t.Expiry)
}

/*
Grant is what a provider returns for a code or a refresh token,
normalized across providers.

Access and Refresh are the tokens that act as the application in the
account, nil when the provider issued none. Scopes lists what the
application was granted. UserAccess, UserRefresh and UserScopes are the
same for the person who approved, when the provider issues a separate
token acting as them, and UserID is who they are at the provider.
Account is the provider's stable id of the connected account, such as a
team or organization, and AccountName its display name. App is the
provider's id of the client application. Raw holds everything else the
provider returned, never a token.
*/
type Grant struct {
	Access      *Token
	Refresh     *Token
	Scopes      []string
	UserAccess  *Token
	UserRefresh *Token
	UserScopes  []string
	UserID      string
	Account     string
	AccountName string
	App         string
	Raw         map[string]any
}

/*
Request is a connection the application wants to start.

Scopes are the application scopes to ask for and UserScopes those acting
as the person, for providers that tell the two apart; empty means the
provider's configured defaults. RedirectURL is where the provider sends
the browser back, empty for the configured one. Extra adds provider
parameters to the authorization URL, such as a team hint. Binding names
the browser session starting the attempt; only the same session may
finish it. Payload is handed back on the callback, so the caller knows
what the attempt was for; it stays on the server.
*/
type Request struct {
	Scopes      []string
	UserScopes  []string
	RedirectURL string
	Extra       map[string]string
	Binding     string
	Payload     map[string]string
}

/*
Attempt is one connection, from the redirect to the callback, as a
driver sees it.

State is echoed back by the provider on the callback. Verifier is the
PKCE secret: a driver whose provider supports PKCE puts only its S256
challenge in the authorization URL and proves it on the exchange.
RedirectURL must be the same on the exchange. Scopes, UserScopes and
Extra are what the request asked for.
*/
type Attempt struct {
	State       string
	Verifier    string
	RedirectURL string
	Scopes      []string
	UserScopes  []string
	Extra       map[string]string
}

/*
Driver is one provider.

A driver must refuse a code that is unknown, already used, or presented
with another redirect URL, or verifier when it sends a challenge, than
it was issued for.
*/
type Driver interface {
	/*
		AuthURL returns the provider URL that asks the person to approve the
		attempt.
	*/
	AuthURL(ctx context.Context, attempt Attempt) (string, error)

	/*
		Exchange trades the code returned for the attempt for a grant.
	*/
	Exchange(ctx context.Context, code string, attempt Attempt) (Grant, error)

	/*
		Refresh trades a refresh token for a fresh grant, or returns
		ErrInvalidGrant when the provider refuses it.
	*/
	Refresh(ctx context.Context, refresh string) (Grant, error)

	/*
		Revoke gives a token back to the provider, or returns ErrUnsupported
		when the provider has no way to.
	*/
	Revoke(ctx context.Context, token string) error
}
