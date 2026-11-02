/*
|--------------------------------------------------------------------------
| OpenID Connect Provider
|--------------------------------------------------------------------------
|
| An enterprise identity provider is named by its issuer alone: its
| endpoints and signing keys come from the issuer's discovery document,
| fetched on first use through the application's HTTP client and kept
| for the life of the process.
|
| Who signed in is read only from the ID token, and the token counts only
| once it is signed by the issuer's published keys, names this client
| as its audience, has not expired, and carries the nonce of this very
| attempt. The address in it is trusted only as far as the provider's
| own email_verified claim says.
|
|   providers:
|     okta:
|       driver: oidc
|       issuer: https://example.okta.com
|       client_id: ...
|       client_secret: ...
|
*/

package oidc

import (
	"context"
	"crypto/subtle"
	"fmt"
	"sync"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/manager"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/socialite/internal/oauth"
)

/*
Options configure an OpenID Connect provider.

Issuer is the provider's issuer URL, exactly as its ID tokens state it.
ClientID and ClientSecret are the registered client; the secret may be
empty for a public client. Scopes default to openid, email and profile.
Prompt, when set, is sent as the prompt parameter, such as
select_account.
*/
type Options struct {
	Issuer       string   `koanf:"issuer"`
	ClientID     string   `koanf:"client_id"`
	ClientSecret string   `koanf:"client_secret"`
	Scopes       []string `koanf:"scopes"`
	Prompt       string   `koanf:"prompt"`
}

/*
Provider signs users in with one OpenID Connect issuer.
*/
type Provider struct {
	name   string
	client *httpclient.Client
	clock  clock.Clock
	opts   Options

	mu       sync.Mutex
	oauth    *oauth.Client
	verifier *gooidc.IDTokenVerifier
}

/*
Factory builds OpenID Connect providers for the socialite manager.
*/
func Factory(client *httpclient.Client, clk clock.Clock) manager.Factory[socialite.Provider] {
	return func(name string, section config.Section) (socialite.Provider, error) {
		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(name, client, clk, opts)
	}
}

/*
New returns the provider configured under the name. Nothing is fetched
until the first sign-in.
*/
func New(name string, client *httpclient.Client, clk clock.Clock, opts Options) (*Provider, error) {
	if opts.Issuer == "" || opts.ClientID == "" {
		return nil, fmt.Errorf("%w: %s needs issuer and client_id", socialite.ErrMisconfigured, name)
	}

	if len(opts.Scopes) == 0 {
		opts.Scopes = []string{gooidc.ScopeOpenID, "email", "profile"}
	}

	return &Provider{name: name, client: client, clock: clk, opts: opts}, nil
}

/*
AuthURL returns the issuer's authorization URL for the attempt, carrying
its nonce.
*/
func (p *Provider) AuthURL(ctx context.Context, attempt socialite.Attempt) (string, error) {
	client, _, err := p.discover(ctx)
	if err != nil {
		return "", err
	}

	params := []oauth2.AuthCodeOption{gooidc.Nonce(attempt.Nonce)}
	if p.opts.Prompt != "" {
		params = append(params, oauth2.SetAuthURLParam("prompt", p.opts.Prompt))
	}

	return client.AuthURL(attempt, params...), nil
}

/*
Exchange trades the code for tokens and verifies the ID token among
them against the attempt.
*/
func (p *Provider) Exchange(ctx context.Context, code string, attempt socialite.Attempt) (socialite.Token, error) {
	client, verifier, err := p.discover(ctx)
	if err != nil {
		return socialite.Token{}, err
	}

	token, err := client.Exchange(ctx, code, attempt)
	if err != nil {
		return socialite.Token{}, err
	}

	token.Claims, err = p.verify(ctx, verifier, token.IDToken, attempt.Nonce)
	if err != nil {
		return socialite.Token{}, err
	}

	return token, nil
}

/*
User reads the user from the verified ID token claims.
*/
func (p *Provider) User(_ context.Context, token socialite.Token) (socialite.User, error) {
	subject, _ := token.Claims["sub"].(string)
	if subject == "" {
		return socialite.User{}, fmt.Errorf("%w: no verified subject", socialite.ErrInvalidIDToken)
	}

	email, _ := token.Claims["email"].(string)
	verified, _ := token.Claims["email_verified"].(bool)
	name, _ := token.Claims["name"].(string)
	picture, _ := token.Claims["picture"].(string)

	return socialite.User{
		Provider:      p.name,
		ID:            subject,
		Email:         email,
		EmailVerified: verified && email != "",
		Name:          name,
		AvatarURL:     picture,
		Raw:           token.Claims,
	}, nil
}

/*
discover fetches the discovery document once and builds the OAuth
client and ID token verifier from it. A failed fetch is retried on the
next sign-in.
*/
func (p *Provider) discover(ctx context.Context) (*oauth.Client, *gooidc.IDTokenVerifier, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.oauth != nil {
		return p.oauth, p.verifier, nil
	}

	issuer, err := gooidc.NewProvider(gooidc.ClientContext(ctx, p.client.StdClient()), p.opts.Issuer)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: discover %s: %w", socialite.ErrProvider, p.opts.Issuer, err)
	}

	p.oauth = &oauth.Client{
		HTTP: p.client,
		Config: oauth2.Config{
			ClientID:     p.opts.ClientID,
			ClientSecret: p.opts.ClientSecret,
			Endpoint:     issuer.Endpoint(),
			Scopes:       p.opts.Scopes,
		},
	}
	p.verifier = issuer.Verifier(&gooidc.Config{ClientID: p.opts.ClientID, Now: p.clock.Now})

	return p.oauth, p.verifier, nil
}

/*
verify checks the ID token and returns its claims. Beyond what the
verifier checks, the nonce must be the attempt's, and a token for
several audiences, or naming the party it was issued to, must name
this client.
*/
func (p *Provider) verify(
	ctx context.Context, verifier *gooidc.IDTokenVerifier, raw, nonce string,
) (map[string]any, error) {
	if raw == "" {
		return nil, fmt.Errorf("%w: token response has none", socialite.ErrInvalidIDToken)
	}

	idToken, err := verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", socialite.ErrInvalidIDToken, err)
	}

	if nonce == "" || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return nil, fmt.Errorf("%w: nonce does not match the attempt", socialite.ErrInvalidIDToken)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("%w: %w", socialite.ErrInvalidIDToken, err)
	}

	party, _ := claims["azp"].(string)
	if (party != "" || len(idToken.Audience) > 1) && party != p.opts.ClientID {
		return nil, fmt.Errorf("%w: issued to %q", socialite.ErrInvalidIDToken, party)
	}

	return claims, nil
}
