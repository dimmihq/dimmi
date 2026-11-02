/*
|--------------------------------------------------------------------------
| Fake Providers
|--------------------------------------------------------------------------
|
| Identity providers live on other people's servers, so application
| tests sign users in through a fake instead. The fake replaces every
| configured provider and plays the provider's part honestly: it hands
| out single-use codes bound to the PKCE challenge and redirect URL of
| the attempt, so the state, PKCE and redirect checks of the real flow
| still run. Approving an authorization URL as a given user stands in
| for that user clicking through the provider's sign-in page.
|
*/

package socialite

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
fakeHost is the host of every fake authorization URL.
*/
const fakeHost = "socialite.test"

/*
Fake is the provider side of every faked provider.
*/
type Fake struct {
	mu     sync.Mutex
	codes  map[string]fakeGrant
	tokens map[string]User
}

/*
fakeGrant is what a fake code was issued for.
*/
type fakeGrant struct {
	provider    string
	challenge   string
	redirectURL string
	user        User
}

/*
Fake replaces every configured provider with a fake and returns it.
It panics with ErrFakesNotAllowed unless the configuration allows fakes:
a fake outside tests would sign anyone in as anyone, which is a
programming error.
*/
func (m *Manager) Fake() *Fake {
	if !m.fakes {
		panic(ErrFakesNotAllowed)
	}

	fake := &Fake{codes: map[string]fakeGrant{}, tokens: map[string]User{}}

	for _, name := range m.providers.Names() {
		m.providers.Swap(name, &fakeProvider{name: name, fake: fake})
	}

	return fake
}

/*
Approve signs the user in at the provider the authorization URL points
to, and returns the callback URL the provider would send the browser
to, carrying a fresh code and the attempt's state.
*/
func (f *Fake) Approve(authURL string, user User) (string, error) {
	parsed, err := url.Parse(authURL)
	if err != nil || parsed.Host != fakeHost {
		return "", fmt.Errorf("%w: %q is not a fake authorization url", ErrProvider, authURL)
	}

	query := parsed.Query()
	provider := strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), "/authorize")

	callback, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || query.Get("code_challenge_method") != "S256" || query.Get("state") == "" {
		return "", fmt.Errorf("%w: incomplete authorization url %q", ErrProvider, authURL)
	}

	user.Provider = provider
	code := str.Random(tokenLength)

	f.mu.Lock()
	f.codes[code] = fakeGrant{
		provider:    provider,
		challenge:   query.Get("code_challenge"),
		redirectURL: callback.String(),
		user:        user,
	}
	f.mu.Unlock()

	values := callback.Query()
	values.Set("code", code)
	values.Set("state", query.Get("state"))
	callback.RawQuery = values.Encode()

	return callback.String(), nil
}

/*
redeem trades a code for an access token, once, when the provider,
verifier and redirect URL match the ones it was issued for.
*/
func (f *Fake) redeem(provider, code string, attempt Attempt) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	grant, ok := f.codes[code]
	delete(f.codes, code)

	if !ok || grant.provider != provider || grant.redirectURL != attempt.RedirectURL ||
		grant.challenge != oauth2.S256ChallengeFromVerifier(attempt.Verifier) {
		return "", fmt.Errorf("%w: invalid_grant", ErrExchange)
	}

	token := str.Random(tokenLength)
	f.tokens[token] = grant.user

	return token, nil
}

func (f *Fake) user(token string) (User, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	user, ok := f.tokens[token]

	return user, ok
}

/*
fakeProvider is one configured provider answered by the fake.
*/
type fakeProvider struct {
	name string
	fake *Fake
}

/*
AuthURL returns a fake authorization URL carrying the attempt.
*/
func (p *fakeProvider) AuthURL(_ context.Context, attempt Attempt) (string, error) {
	cfg := oauth2.Config{
		ClientID:    "fake-" + p.name,
		RedirectURL: attempt.RedirectURL,
		Endpoint:    oauth2.Endpoint{AuthURL: "https://" + fakeHost + "/" + p.name + "/authorize"},
	}

	return cfg.AuthCodeURL(attempt.State,
		oauth2.S256ChallengeOption(attempt.Verifier),
		oauth2.SetAuthURLParam("nonce", attempt.Nonce),
	), nil
}

/*
Exchange redeems a code the fake issued.
*/
func (p *fakeProvider) Exchange(_ context.Context, code string, attempt Attempt) (Token, error) {
	token, err := p.fake.redeem(p.name, code, attempt)
	if err != nil {
		return Token{}, err
	}

	return Token{AccessToken: token}, nil
}

/*
User returns the user the access token was issued to.
*/
func (p *fakeProvider) User(_ context.Context, token Token) (User, error) {
	user, ok := p.fake.user(token.AccessToken)
	if !ok {
		return User{}, fmt.Errorf("%w: unknown access token", ErrProvider)
	}

	return user, nil
}
