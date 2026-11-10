/*
|--------------------------------------------------------------------------
| Fake Providers
|--------------------------------------------------------------------------
|
| Providers live on other people's servers, so application tests connect
| accounts through a fake instead. The fake replaces every configured
| provider and plays the provider's part honestly: it hands out
| single-use codes bound to the redirect URL and PKCE challenge of the
| attempt, so the state, session and redirect checks of the real flow
| still run. Approving an authorization URL with a grant stands in for
| the person clicking through the provider's consent page.
|
| Refresh tokens the test registers answer with the grant it gave, and
| every refresh and revocation is counted, so a test can prove a token
| was renewed once or given back.
|
*/

package oauthclient

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"

	"golang.org/x/oauth2"

	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
fakeHost is the host of every fake authorization URL.
*/
const fakeHost = "oauthclient.test"

/*
Fake is the provider side of every faked provider.
*/
type Fake struct {
	mu        sync.Mutex
	codes     map[string]fakeCode
	refreshes map[string]Grant
	refreshed []string
	revoked   []string
}

/*
fakeCode is what a fake code was issued for.
*/
type fakeCode struct {
	provider    string
	challenge   string
	redirectURL string
	grant       Grant
}

/*
Fake replaces every configured provider with a fake and returns it.
It panics with ErrFakesNotAllowed unless the configuration allows fakes:
a fake outside tests would connect any account to anyone, which is a
programming error.
*/
func (m *Manager) Fake() *Fake {
	if !m.fakes {
		panic(ErrFakesNotAllowed)
	}

	fake := &Fake{codes: map[string]fakeCode{}, refreshes: map[string]Grant{}}

	for _, name := range m.providers.Names() {
		m.providers.Swap(name, &fakeDriver{name: name, fake: fake})
	}

	return fake
}

/*
Approve grants the attempt the authorization URL carries, and returns
the callback URL the provider would send the browser to, carrying a
fresh code and the attempt's state. The code is traded for the grant;
a grant without any token gets a fresh application token.
*/
func (f *Fake) Approve(authURL string, grant Grant) (string, error) {
	parsed, err := parseFakeURL(authURL)
	if err != nil {
		return "", err
	}

	if grant.Access == nil && grant.UserAccess == nil {
		grant.Access = &Token{Value: "fake-" + str.Random(stateLength)}
	}

	code := str.Random(stateLength)

	f.mu.Lock()
	f.codes[code] = fakeCode{
		provider: parsed.provider, challenge: parsed.query.Get("code_challenge"),
		redirectURL: parsed.callback.String(), grant: grant,
	}
	f.mu.Unlock()

	return withQuery(parsed.callback, url.Values{"code": {code}, "state": {parsed.query.Get("state")}}), nil
}

/*
Deny returns the callback URL the provider would send the browser to
when the person declines the attempt.
*/
func (f *Fake) Deny(authURL string) (string, error) {
	parsed, err := parseFakeURL(authURL)
	if err != nil {
		return "", err
	}

	return withQuery(parsed.callback, url.Values{"error": {"access_denied"}, "state": {parsed.query.Get("state")}}), nil
}

/*
AcceptRefresh makes the refresh token answer with the grant until it
is refreshed once; refresh tokens never registered are refused with
ErrInvalidGrant.
*/
func (f *Fake) AcceptRefresh(refresh string, grant Grant) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.refreshes[refresh] = grant
}

/*
Refreshed returns every refresh token presented, oldest first.
*/
func (f *Fake) Refreshed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.refreshed)
}

/*
Revoked returns every token given back, oldest first.
*/
func (f *Fake) Revoked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.revoked)
}

/*
redeem trades a code for its grant, once, when the provider, redirect
URL and verifier match the ones it was issued for.
*/
func (f *Fake) redeem(provider, code string, attempt Attempt) (Grant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	issued, ok := f.codes[code]
	delete(f.codes, code)

	if !ok || issued.provider != provider || issued.redirectURL != attempt.RedirectURL ||
		issued.challenge != oauth2.S256ChallengeFromVerifier(attempt.Verifier) {
		return Grant{}, fmt.Errorf("%w: invalid_grant", ErrExchange)
	}

	return issued.grant, nil
}

func (f *Fake) refresh(refresh string) (Grant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.refreshed = append(f.refreshed, refresh)

	grant, ok := f.refreshes[refresh]
	delete(f.refreshes, refresh)

	if !ok {
		return Grant{}, fmt.Errorf("%w: invalid_grant", ErrInvalidGrant)
	}

	return grant, nil
}

func (f *Fake) revoke(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.revoked = append(f.revoked, token)
}

/*
fakeURL is a fake authorization URL read back: the provider it points
to, its query and the callback URL it names.
*/
type fakeURL struct {
	provider string
	query    url.Values
	callback *url.URL
}

/*
parseFakeURL reads a fake authorization URL.
*/
func parseFakeURL(authURL string) (fakeURL, error) {
	parsed, err := url.Parse(authURL)
	if err != nil || parsed.Host != fakeHost {
		return fakeURL{}, fmt.Errorf("%w: %q is not a fake authorization url", ErrProvider, authURL)
	}

	query := parsed.Query()

	callback, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || query.Get("state") == "" || query.Get("code_challenge_method") != "S256" {
		return fakeURL{}, fmt.Errorf("%w: incomplete authorization url %q", ErrProvider, authURL)
	}

	provider := strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), "/authorize")

	return fakeURL{provider: provider, query: query, callback: callback}, nil
}

/*
withQuery returns the URL with the values added to its query.
*/
func withQuery(target *url.URL, add url.Values) string {
	values := target.Query()
	maps.Copy(values, add)

	out := *target
	out.RawQuery = values.Encode()

	return out.String()
}

/*
fakeDriver is one configured provider answered by the fake.
*/
type fakeDriver struct {
	name string
	fake *Fake
}

/*
AuthURL returns a fake authorization URL carrying the attempt, the
scopes it asks for and its extra parameters.
*/
func (d *fakeDriver) AuthURL(_ context.Context, attempt Attempt) (string, error) {
	cfg := oauth2.Config{
		ClientID:    "fake-" + d.name,
		RedirectURL: attempt.RedirectURL,
		Scopes:      attempt.Scopes,
		Endpoint:    oauth2.Endpoint{AuthURL: "https://" + fakeHost + "/" + d.name + "/authorize"},
	}

	opts := []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(attempt.Verifier)}
	if len(attempt.UserScopes) > 0 {
		opts = append(opts, oauth2.SetAuthURLParam("user_scope", strings.Join(attempt.UserScopes, " ")))
	}

	for key, value := range attempt.Extra {
		opts = append(opts, oauth2.SetAuthURLParam(key, value))
	}

	return cfg.AuthCodeURL(attempt.State, opts...), nil
}

/*
Exchange redeems a code the fake issued.
*/
func (d *fakeDriver) Exchange(_ context.Context, code string, attempt Attempt) (Grant, error) {
	return d.fake.redeem(d.name, code, attempt)
}

/*
Refresh answers with the grant registered for the refresh token.
*/
func (d *fakeDriver) Refresh(_ context.Context, refresh string) (Grant, error) {
	return d.fake.refresh(refresh)
}

/*
Revoke records the token as given back.
*/
func (d *fakeDriver) Revoke(_ context.Context, token string) error {
	d.fake.revoke(token)

	return nil
}
