/*
|--------------------------------------------------------------------------
| Slack Provider
|--------------------------------------------------------------------------
|
| Slack installs an app into a Slack team with its own flavor of
| OAuth 2. Scopes are joined by commas, and the scopes acting as the
| person who approves go in a separate user_scope parameter. The token
| exchange answers HTTP 200 even when it refuses, with ok set to false,
| and returns two tokens at once: the bot token at the top level and the
| approver's own token under authed_user. The Slack team is the
| connected account, and an Enterprise Grid organization install is
| reported in Raw for the caller to judge.
|
|   providers:
|     slack:
|       driver: slack
|       client_id: ...
|       client_secret: ...
|
*/

package slack

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/oauthclient"
)

/*
Options configure the Slack provider.

ClientID and ClientSecret are the Slack app's. Scopes and UserScopes are
asked for when an attempt names none. AuthURL and APIURL default to
slack.com.
*/
type Options struct {
	ClientID     string   `koanf:"client_id"`
	ClientSecret string   `koanf:"client_secret"`
	Scopes       []string `koanf:"scopes"`
	UserScopes   []string `koanf:"user_scopes"`
	AuthURL      string   `default:"https://slack.com/oauth/v2/authorize" koanf:"auth_url"`
	APIURL       string   `default:"https://slack.com/api"                koanf:"api_url"`
}

/*
Driver installs a Slack app.
*/
type Driver struct {
	opts   Options
	client *httpclient.Client
	clock  clock.Clock
}

/*
Factory builds Slack providers for the oauthclient manager.
*/
func Factory(client *httpclient.Client, clk clock.Clock) manager.Factory[oauthclient.Driver] {
	return func(name string, section config.Section) (oauthclient.Driver, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(name, client, clk, opts)
	}
}

/*
New returns the Slack provider configured under the name, reading token
expiries against the clock.
*/
func New(name string, client *httpclient.Client, clk clock.Clock, opts Options) (*Driver, error) {
	if opts.ClientID == "" || opts.ClientSecret == "" {
		return nil, fmt.Errorf("%w: %s needs client_id and client_secret", oauthclient.ErrMisconfigured, name)
	}

	opts.APIURL = strings.TrimRight(opts.APIURL, "/")

	return &Driver{opts: opts, client: client, clock: clk}, nil
}

/*
AuthURL returns Slack's install URL for the attempt.
*/
func (d *Driver) AuthURL(_ context.Context, attempt oauthclient.Attempt) (string, error) {
	scopes, userScopes := attempt.Scopes, attempt.UserScopes
	if len(scopes) == 0 && len(userScopes) == 0 {
		scopes, userScopes = d.opts.Scopes, d.opts.UserScopes
	}

	query := url.Values{
		"client_id": {d.opts.ClientID}, "redirect_uri": {attempt.RedirectURL}, "state": {attempt.State},
	}

	for key, value := range attempt.Extra {
		query.Set(key, value)
	}

	if len(scopes) > 0 {
		query.Set("scope", strings.Join(scopes, ","))
	}

	if len(userScopes) > 0 {
		query.Set("user_scope", strings.Join(userScopes, ","))
	}

	return d.opts.AuthURL + "?" + query.Encode(), nil
}

/*
Exchange trades the code for the bot and approver tokens. Slack refusing
the code is ErrExchange.
*/
func (d *Driver) Exchange(ctx context.Context, code string, attempt oauthclient.Attempt) (oauthclient.Grant, error) {
	return d.access(ctx, url.Values{"code": {code}, "redirect_uri": {attempt.RedirectURL}}, oauthclient.ErrExchange)
}

/*
Refresh trades a refresh token of an app with token rotation on for
fresh tokens. Slack refusing it is ErrInvalidGrant.
*/
func (d *Driver) Refresh(ctx context.Context, refresh string) (oauthclient.Grant, error) {
	grant, err := d.access(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}},
		oauthclient.ErrInvalidGrant)
	if err != nil {
		return oauthclient.Grant{}, err
	}

	if grant.Refresh == nil && grant.Access != nil {
		grant.Refresh = &oauthclient.Token{Value: refresh}
	}

	return grant, nil
}

/*
Revoke gives the token back with auth.revoke. A token Slack no longer
knows is already given back.
*/
func (d *Driver) Revoke(ctx context.Context, token string) error {
	var answer struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}

	if err := d.call(ctx, apiCall{method: "auth.revoke", token: token}, &answer); err != nil {
		return err
	}

	if answer.OK || answer.Error == "invalid_auth" || answer.Error == "token_revoked" ||
		answer.Error == "account_inactive" {
		return nil
	}

	return fmt.Errorf("%w: auth.revoke: %s", oauthclient.ErrProvider, answer.Error)
}

/*
access calls oauth.v2.access with the client's credentials and reads
its answer into a grant; a refusal is the refused error.
*/
func (d *Driver) access(ctx context.Context, form url.Values, refused error) (oauthclient.Grant, error) {
	form.Set("client_id", d.opts.ClientID)
	form.Set("client_secret", d.opts.ClientSecret)

	var answer accessAnswer
	if err := d.call(ctx, apiCall{method: "oauth.v2.access", form: form}, &answer); err != nil {
		return oauthclient.Grant{}, err
	}

	if !answer.OK {
		return oauthclient.Grant{}, fmt.Errorf("%w: %s", refused, answer.Error)
	}

	return answer.grant(d.clock.Now()), nil
}

/*
apiCall is one Web API call: the method, the token it authenticates
with, if any, and the form it posts.
*/
type apiCall struct {
	method string
	token  string
	form   url.Values
}

/*
call posts the form to the Web API method and decodes the answer. The
answer's raw fields are kept in it when it has room for them.
*/
func (d *Driver) call(ctx context.Context, c apiCall, into any) error {
	method := c.method
	req := d.client.New().WithHeader("Content-Type", "application/x-www-form-urlencoded")
	if c.token != "" {
		req = req.WithToken(c.token)
	}

	resp, err := req.Post(ctx, d.opts.APIURL+"/"+method, c.form.Encode())
	if err != nil {
		return fmt.Errorf("%w: %s: %w", oauthclient.ErrProvider, method, err)
	}

	if resp.Status != http.StatusOK {
		return fmt.Errorf("%w: %s answered %d", oauthclient.ErrProvider, method, resp.Status)
	}

	if err := resp.JSON(into); err != nil {
		return fmt.Errorf("%w: %s: %w", oauthclient.ErrProvider, method, err)
	}

	if keeper, ok := into.(rawKeeper); ok {
		return keeper.keep(resp.Body)
	}

	return nil
}
