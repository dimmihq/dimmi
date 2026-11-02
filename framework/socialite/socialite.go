/*
|--------------------------------------------------------------------------
| Social Login
|--------------------------------------------------------------------------
|
| A user may sign in with an account they already hold at an identity
| provider. Every provider speaks the authorization code flow, so a
| driver only says where to send the browser, how to trade the returned
| code for tokens, and how to read the user from them. Whatever the
| provider, the application gets the same normalized user back.
|
| Each sign-in attempt carries its own state, PKCE verifier and nonce.
| The state ties the callback to the browser that started the attempt,
| the verifier ties the code to this server, and the nonce ties an ID
| token to this attempt, so a code or token captured elsewhere is
| worthless here.
|
*/

package socialite

import (
	"context"
	"errors"
	"time"
)

/*
Errors from a sign-in.

ErrInvalidState refuses a callback whose state is missing, unknown,
expired, already used, or started by another browser or for another
provider. ErrDenied means the user declined at the provider. ErrProvider
covers any other failure the provider reports or causes, such as an
error on the callback or an unreachable API. ErrExchange means the
provider refused the code. ErrInvalidIDToken refuses an ID token that is
missing, unsigned by the provider, meant for another client, expired or
issued for another attempt. ErrRedirectNotAllowed refuses a redirect URL
the provider's configuration does not list. ErrMisconfigured comes from
a provider whose configuration lacks a required option.
*/
var (
	ErrInvalidState       = errors.New("socialite: invalid state")
	ErrDenied             = errors.New("socialite: sign-in denied at the provider")
	ErrProvider           = errors.New("socialite: provider error")
	ErrExchange           = errors.New("socialite: code exchange refused")
	ErrInvalidIDToken     = errors.New("socialite: invalid id token")
	ErrRedirectNotAllowed = errors.New("socialite: redirect url not allowed")
	ErrMisconfigured      = errors.New("socialite: provider misconfigured")
)

/*
ErrFakesNotAllowed is what Fake panics with on a manager whose
configuration does not allow fakes.
*/
var ErrFakesNotAllowed = errors.New("socialite: fakes are allowed only in tests")

/*
User is the person a provider signed in, normalized across providers.

Provider is the configured name of the provider, and ID the provider's
stable identifier for the person; the pair identifies an account. Email
is the address the provider holds, and EmailVerified says whether the
provider vouches that the person controls it; an unverified address
proves nothing. Name and AvatarURL may be empty. Raw holds everything
the provider returned about the person. Token holds the tokens the code
was traded for.
*/
type User struct {
	Provider      string
	ID            string
	Email         string
	EmailVerified bool
	Name          string
	AvatarURL     string
	Raw           map[string]any
	Token         Token
}

/*
Token is what a provider returns for a code.

AccessToken calls the provider's API on the user's behalf until Expiry,
and RefreshToken, when the provider issues one, gets a new one. IDToken
is the raw OpenID Connect ID token, and Claims its claims once the
driver has verified it; both are empty for plain OAuth 2 providers.
*/
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	IDToken      string
	Claims       map[string]any
}

/*
Attempt is one sign-in, from the redirect to the callback.

State is echoed back by the provider on the callback. Verifier is the
PKCE secret: the authorization URL carries only its S256 challenge and
the exchange proves the caller holds it. Nonce is echoed inside the ID token
by OpenID Connect providers. RedirectURL is where the provider sends the
browser back, and must be the same on the exchange.
*/
type Attempt struct {
	State       string
	Verifier    string
	Nonce       string
	RedirectURL string
}

/*
Provider is one identity provider.

A provider must refuse a code that is unknown, already used, or
presented with another verifier or redirect URL than it was issued for.
*/
type Provider interface {
	/*
		AuthURL returns the provider URL that asks the user to sign in for
		the attempt.
	*/
	AuthURL(ctx context.Context, attempt Attempt) (string, error)

	/*
		Exchange trades the code returned for the attempt for tokens,
		verifying any ID token against the attempt.
	*/
	Exchange(ctx context.Context, code string, attempt Attempt) (Token, error)

	/*
		User reads the signed-in user from the tokens.
	*/
	User(ctx context.Context, token Token) (User, error)
}
