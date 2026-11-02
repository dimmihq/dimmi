/*
|--------------------------------------------------------------------------
| GitHub Provider
|--------------------------------------------------------------------------
|
| GitHub speaks plain OAuth 2 without ID tokens, so the user is read from
| its API with the access token. The address on a GitHub profile is
| whatever the user chose to show and proves nothing, so the email is the
| account's primary address from the emails API, marked verified only
| when GitHub has verified it.
|
| The endpoints are configurable for GitHub Enterprise Server.
|
|   providers:
|     github:
|       driver: github
|       client_id: ...
|       client_secret: ...
|
*/

package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/internal/oauth"
)

/*
apiVersion is the GitHub REST API version the responses are read as.
*/
const apiVersion = "2022-11-28"

/*
Options configure the GitHub provider.

ClientID and ClientSecret are the registered OAuth app. Scopes default
to read:user and user:email, the least that reads the profile and the
account's addresses. AuthURL, TokenURL and APIURL default to github.com.
*/
type Options struct {
	ClientID     string   `koanf:"client_id"`
	ClientSecret string   `koanf:"client_secret"`
	Scopes       []string `koanf:"scopes"`
	AuthURL      string   `default:"https://github.com/login/oauth/authorize"    koanf:"auth_url"`
	TokenURL     string   `default:"https://github.com/login/oauth/access_token" koanf:"token_url"`
	APIURL       string   `default:"https://api.github.com"                      koanf:"api_url"`
}

/*
Provider signs users in with GitHub.
*/
type Provider struct {
	name   string
	client *httpclient.Client
	oauth  oauth.Client
	apiURL string
}

/*
Factory builds GitHub providers for the socialite manager.
*/
func Factory(client *httpclient.Client) manager.Factory[socialite.Provider] {
	return func(name string, section config.Section) (socialite.Provider, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(name, client, opts)
	}
}

/*
New returns the GitHub provider configured under the name.
*/
func New(name string, client *httpclient.Client, opts Options) (*Provider, error) {
	if opts.ClientID == "" || opts.ClientSecret == "" {
		return nil, fmt.Errorf("%w: %s needs client_id and client_secret", socialite.ErrMisconfigured, name)
	}

	if len(opts.Scopes) == 0 {
		opts.Scopes = []string{"read:user", "user:email"}
	}

	return &Provider{
		name:   name,
		client: client,
		apiURL: opts.APIURL,
		oauth: oauth.Client{
			HTTP: client,
			Config: oauth2.Config{
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				Scopes:       opts.Scopes,
				Endpoint: oauth2.Endpoint{
					AuthURL:   opts.AuthURL,
					TokenURL:  opts.TokenURL,
					AuthStyle: oauth2.AuthStyleInParams,
				},
			},
		},
	}, nil
}

/*
AuthURL returns GitHub's authorization URL for the attempt.
*/
func (p *Provider) AuthURL(_ context.Context, attempt socialite.Attempt) (string, error) {
	return p.oauth.AuthURL(attempt), nil
}

/*
Exchange trades the code for an access token.
*/
func (p *Provider) Exchange(ctx context.Context, code string, attempt socialite.Attempt) (socialite.Token, error) {
	return p.oauth.Exchange(ctx, code, attempt)
}

/*
profile is the part of a GitHub user the provider reads.
*/
type profile struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

/*
address is one entry of the emails API.
*/
type address struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

/*
User reads the profile and the primary address. A user without a name
is named by their login.
*/
func (p *Provider) User(ctx context.Context, token socialite.Token) (socialite.User, error) {
	var (
		raw    map[string]any
		person profile
	)

	if err := p.get(ctx, token.AccessToken, "/user", &raw, &person); err != nil {
		return socialite.User{}, err
	}

	if person.ID == 0 {
		return socialite.User{}, fmt.Errorf("%w: github user has no id", socialite.ErrProvider)
	}

	var addresses []address
	if err := p.get(ctx, token.AccessToken, "/user/emails", &addresses); err != nil {
		return socialite.User{}, err
	}

	user := socialite.User{
		Provider:  p.name,
		ID:        strconv.FormatInt(person.ID, 10),
		Name:      person.Name,
		AvatarURL: person.AvatarURL,
		Raw:       raw,
	}

	if user.Name == "" {
		user.Name = person.Login
	}

	for _, a := range addresses {
		if a.Primary {
			user.Email, user.EmailVerified = a.Email, a.Verified
		}
	}

	return user, nil
}

/*
get reads the API path and decodes the answer into every target.
*/
func (p *Provider) get(ctx context.Context, token, path string, targets ...any) error {
	resp, err := p.client.New().
		WithToken(token).
		Accept("application/vnd.github+json").
		WithHeader("X-GitHub-Api-Version", apiVersion).
		Get(ctx, p.apiURL+path)
	if err != nil {
		return fmt.Errorf("%w: %w", socialite.ErrProvider, err)
	}

	if resp.Status != http.StatusOK {
		return fmt.Errorf("%w: github %s answered %d", socialite.ErrProvider, path, resp.Status)
	}

	for _, target := range targets {
		if err := resp.JSON(target); err != nil {
			return fmt.Errorf("%w: github %s: %w", socialite.ErrProvider, path, err)
		}
	}

	return nil
}
