/*
|--------------------------------------------------------------------------
| Generic Provider
|--------------------------------------------------------------------------
|
| Most services follow the OAuth 2 standard closely enough that their
| endpoints are all a driver needs: where to send the browser, where to
| trade codes and refresh tokens, and where to give tokens back. PKCE is
| on unless the provider refuses it, and scopes are joined by spaces
| unless the provider wants another separator.
|
|   providers:
|     tracker:
|       driver: generic
|       client_id: ...
|       client_secret: ...
|       auth_url: https://tracker.example.com/oauth/authorize
|       token_url: https://api.tracker.example.com/oauth/token
|       revoke_url: https://api.tracker.example.com/oauth/revoke
|       scopes: [read, write]
|
*/

package generic

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/oauthclient/internal/flow"
)

/*
Options configure a generic provider.

ClientID and ClientSecret are the registered client. AuthURL and
TokenURL are required; RevokeURL is optional, and without it tokens
cannot be given back. Scopes are asked for when an attempt names none,
joined by ScopeSeparator. PKCE sends a challenge, on by default.
HeaderAuth sends the client credentials as HTTP basic auth rather than
in the form.
*/
type Options struct {
	ClientID       string   `koanf:"client_id"`
	ClientSecret   string   `koanf:"client_secret"`
	AuthURL        string   `koanf:"auth_url"`
	TokenURL       string   `koanf:"token_url"`
	RevokeURL      string   `koanf:"revoke_url"`
	Scopes         []string `koanf:"scopes"`
	ScopeSeparator string   `koanf:"scope_separator"`
	PKCE           bool     `default:"true" koanf:"pkce"`
	HeaderAuth     bool     `koanf:"header_auth"`
}

/*
Driver connects accounts at a provider that follows the standard.
*/
type Driver struct {
	flow      flow.Client
	revokeURL string
}

/*
Factory builds generic providers for the oauthclient manager.
*/
func Factory(client *httpclient.Client) manager.Factory[oauthclient.Driver] {
	return func(name string, section config.Section) (oauthclient.Driver, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(name, client, opts)
	}
}

/*
New returns the generic provider configured under the name.
*/
func New(name string, client *httpclient.Client, opts Options) (*Driver, error) {
	if opts.ClientID == "" || opts.ClientSecret == "" || opts.AuthURL == "" || opts.TokenURL == "" {
		return nil, fmt.Errorf("%w: %s needs client_id, client_secret, auth_url and token_url",
			oauthclient.ErrMisconfigured, name)
	}

	style := oauth2.AuthStyleInParams
	if opts.HeaderAuth {
		style = oauth2.AuthStyleInHeader
	}

	return &Driver{
		revokeURL: opts.RevokeURL,
		flow: flow.Client{
			HTTP: client, PKCE: opts.PKCE, ScopeSeparator: opts.ScopeSeparator,
			Config: oauth2.Config{
				ClientID: opts.ClientID, ClientSecret: opts.ClientSecret, Scopes: opts.Scopes,
				Endpoint: oauth2.Endpoint{AuthURL: opts.AuthURL, TokenURL: opts.TokenURL, AuthStyle: style},
			},
		},
	}, nil
}

/*
AuthURL returns the provider's authorization URL for the attempt.
*/
func (d *Driver) AuthURL(_ context.Context, attempt oauthclient.Attempt) (string, error) {
	return d.flow.AuthURL(attempt), nil
}

/*
Exchange trades the code for a grant.
*/
func (d *Driver) Exchange(ctx context.Context, code string, attempt oauthclient.Attempt) (oauthclient.Grant, error) {
	return d.flow.Exchange(ctx, code, attempt)
}

/*
Refresh trades the refresh token for a fresh grant.
*/
func (d *Driver) Refresh(ctx context.Context, refresh string) (oauthclient.Grant, error) {
	return d.flow.Refresh(ctx, refresh)
}

/*
Revoke gives the token back at the revocation URL.
*/
func (d *Driver) Revoke(ctx context.Context, token string) error {
	return d.flow.Revoke(ctx, d.revokeURL, token)
}
