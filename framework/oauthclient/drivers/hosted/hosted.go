/*
|--------------------------------------------------------------------------
| Hosted Provider
|--------------------------------------------------------------------------
|
| Some services give every customer their own host, and their OAuth
| endpoints live there: the browser goes to acme.example.com to agree,
| and the code is traded at acme.example.com too. The endpoints are
| written with an {account} placeholder, the attempt names the account
| in its extra "account" parameter, and the grant comes back with that
| account, so the connection knows where it points. An account is a
| plain host label, so an attempt can never send a code anywhere else.
| A refresh token carries its account in front of it, since a refresh
| is asked with the token alone and must still reach the right host.
|
|   providers:
|     helpdesk:
|       driver: hosted
|       client_id: ...
|       client_secret: ...
|       auth_url: https://{account}.helpdesk.example/oauth/authorize
|       token_url: https://{account}.helpdesk.example/oauth/token
|       scopes: [read]
|
*/

package hosted

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/oauthclient/internal/flow"
)

/*
AccountParam is the attempt's extra parameter naming the account.
*/
const AccountParam = "account"

/*
ErrAccount refuses an attempt without a valid account.
*/
var ErrAccount = errors.New("oauthclient: the attempt names no valid account")

/*
accountPattern is a host label.
*/
var accountPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

/*
Options configure a hosted provider: the registered client, endpoint
templates holding {account}, default scopes, whether PKCE is sent, on
by default, and whether the client authenticates with HTTP basic auth.
*/
type Options struct {
	ClientID     string   `koanf:"client_id"`
	ClientSecret string   `koanf:"client_secret"`
	AuthURL      string   `koanf:"auth_url"`
	TokenURL     string   `koanf:"token_url"`
	Scopes       []string `koanf:"scopes"`
	PKCE         bool     `default:"true" koanf:"pkce"`
	HeaderAuth   bool     `koanf:"header_auth"`
}

/*
Driver connects accounts at a provider hosted per customer.
*/
type Driver struct {
	opts Options
	http *httpclient.Client
}

/*
Factory builds hosted providers for the oauthclient manager.
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
New returns the hosted provider configured under the name.
*/
func New(name string, client *httpclient.Client, opts Options) (*Driver, error) {
	if opts.ClientID == "" || !strings.Contains(opts.AuthURL, "{account}") ||
		!strings.Contains(opts.TokenURL, "{account}") {
		return nil, fmt.Errorf("%w: %s needs client_id and auth_url and token_url holding {account}",
			oauthclient.ErrMisconfigured, name)
	}

	return &Driver{opts: opts, http: client}, nil
}

/*
flowFor is the standard flow at the account's endpoints.
*/
func (d *Driver) flowFor(account string) flow.Client {
	style := oauth2.AuthStyleInParams
	if d.opts.HeaderAuth {
		style = oauth2.AuthStyleInHeader
	}

	return flow.Client{
		HTTP: d.http, PKCE: d.opts.PKCE,
		Config: oauth2.Config{
			ClientID: d.opts.ClientID, ClientSecret: d.opts.ClientSecret, Scopes: d.opts.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:   strings.ReplaceAll(d.opts.AuthURL, "{account}", account),
				TokenURL:  strings.ReplaceAll(d.opts.TokenURL, "{account}", account),
				AuthStyle: style,
			},
		},
	}
}

/*
account reads and checks the attempt's account, returning the attempt
without it, so it is not sent as a parameter.
*/
func account(attempt oauthclient.Attempt) (string, oauthclient.Attempt, error) {
	name := strings.ToLower(strings.TrimSpace(attempt.Extra[AccountParam]))
	if !accountPattern.MatchString(name) {
		return "", attempt, ErrAccount
	}

	attempt.Extra = maps.Clone(attempt.Extra)
	delete(attempt.Extra, AccountParam)

	return name, attempt, nil
}

/*
AuthURL returns the account's authorization URL for the attempt.
*/
func (d *Driver) AuthURL(_ context.Context, attempt oauthclient.Attempt) (string, error) {
	name, rest, err := account(attempt)
	if err != nil {
		return "", err
	}

	return d.flowFor(name).AuthURL(rest), nil
}

/*
Exchange trades the code at the account's token endpoint and names the
account on the grant.
*/
func (d *Driver) Exchange(ctx context.Context, code string, attempt oauthclient.Attempt) (oauthclient.Grant, error) {
	name, rest, err := account(attempt)
	if err != nil {
		return oauthclient.Grant{}, err
	}

	grant, err := d.flowFor(name).Exchange(ctx, code, rest)
	if err != nil {
		return grant, err
	}

	grant.Account = name

	return carry(grant, name), nil
}

/*
Refresh trades a refresh token at the token endpoint of the account
it carries, and names the account on the fresh grant.
*/
func (d *Driver) Refresh(ctx context.Context, refresh string) (oauthclient.Grant, error) {
	name, token, ok := strings.Cut(refresh, " ")
	if !ok || !accountPattern.MatchString(name) {
		return oauthclient.Grant{}, oauthclient.ErrInvalidGrant
	}

	grant, err := d.flowFor(name).Refresh(ctx, token)
	if err != nil {
		return grant, err
	}

	grant.Account = name

	return carry(grant, name), nil
}

/*
carry puts the account in front of the grant's refresh token.
*/
func carry(grant oauthclient.Grant, account string) oauthclient.Grant {
	if grant.Refresh != nil && grant.Refresh.Value != "" {
		refresh := *grant.Refresh
		refresh.Value = account + " " + refresh.Value
		grant.Refresh = &refresh
	}

	return grant
}

/*
Revoke is unsupported: the token alone names no account to reach.
*/
func (d *Driver) Revoke(context.Context, string) error {
	return oauthclient.ErrUnsupported
}
