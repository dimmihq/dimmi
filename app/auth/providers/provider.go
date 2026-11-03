/*
|--------------------------------------------------------------------------
| Identity Providers
|--------------------------------------------------------------------------
|
| A person may prove who they are with an account they hold elsewhere.
| Every provider is a driver behind one interface, found by its key, so
| the sign-in flow never names a provider: adding one means writing a
| driver and configuring it, never touching the flow.
|
| Whatever the provider, it hands back the same identity. The provider's
| own id for the person is the identity; the address it reports is only
| a claim, trusted as far as the provider says it verified it.
|
*/

package providers

import (
	"context"
	"errors"
	"net/url"
)

/*
The protocols a provider speaks.
*/
const (
	ProtocolOAuth2 = "oauth2"
	ProtocolOIDC   = "oidc"
)

/*
Errors from a provider.

ErrUnknownProvider means no driver has the key, or its driver is not
configured on this install. ErrState refuses a callback that does not
belong to an attempt this browser started. ErrDenied means the person
declined at the provider. ErrUnavailable covers every other failure
of the provider, such as a refused code or an unreachable API.
*/
var (
	ErrUnknownProvider = errors.New("providers: unknown provider")
	ErrState           = errors.New("providers: invalid state")
	ErrDenied          = errors.New("providers: denied at the provider")
	ErrUnavailable     = errors.New("providers: provider unavailable")
)

/*
Display is how apps show a provider on their sign-in screens.
*/
type Display struct {
	Name string
}

/*
Authorization is where to send the browser to sign in, and the state
the browser keeps until the provider sends it back.
*/
type Authorization struct {
	URL   string
	State string
}

/*
ExternalIdentity is the person a provider signed in.

Provider is the provider's key and Subject its stable id for the
person; the pair is the identity. Email is the address the provider
holds, and EmailVerified whether it vouches the person controls it.
Name, Username, AvatarURL and HostedDomain, the domain of a managed
account, may be empty.
*/
type ExternalIdentity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Username      string
	AvatarURL     string
	HostedDomain  string
}

/*
IdentityProvider is one way of signing in with an outside account.
*/
type IdentityProvider interface {
	/*
		Key names the provider in routes, configuration and stored rows,
		such as "github".
	*/
	Key() string

	/*
		Protocol is the protocol the provider speaks, one of the
		protocols above.
	*/
	Protocol() string

	/*
		Display is how apps show the provider.
	*/
	Display() Display

	/*
		AuthorizeURL starts a sign-in attempt.
	*/
	AuthorizeURL(ctx context.Context) (Authorization, error)

	/*
		Callback finishes the attempt the state belongs to, from the query
		the provider sent the browser back with. Errors: ErrState,
		ErrDenied, ErrUnavailable.
	*/
	Callback(ctx context.Context, state string, query url.Values) (ExternalIdentity, error)
}

/*
ProviderResolver finds the providers this install offers.
*/
type ProviderResolver interface {
	/*
		Find returns the provider with the key, or ErrUnknownProvider.
	*/
	Find(ctx context.Context, key string) (IdentityProvider, error)

	/*
		Offered returns every provider the install offers, in the order
		apps list them.
	*/
	Offered(ctx context.Context) []IdentityProvider
}
