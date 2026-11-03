/*
|--------------------------------------------------------------------------
| Identity Providers In Tests
|--------------------------------------------------------------------------
|
| Identity providers live on other people's servers, so tests sign in
| through the framework's fake provider, which still checks the state,
| the PKCE verifier and the callback address of every attempt. A test
| starts an attempt the way a browser does, approves it as the person
| at the provider, and follows the callback with the cookie the browser
| kept.
|
*/

package tests

import (
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
OAuthAttempt is a sign-in a browser started at a provider: the provider
URL it was sent to and the cookie it keeps until it comes back.
*/
type OAuthAttempt struct {
	AuthURL string
	Cookie  *fhttp.Cookie
}

/*
Social swaps every configured identity provider for the fake, for the
rest of the test, and returns it. Every call returns the same fake.
*/
func (tc *TestCase) Social() *socialite.Fake {
	tc.oauthOnce.Do(func() { tc.social = tc.App.Social.Fake() })

	return tc.social
}

/*
StartOAuth opens the redirect at the path, such as
/auth/github/redirect, with the client, and returns the attempt. Call
Social first to sign in through the fake rather than the real provider.
*/
func (tc *TestCase) StartOAuth(client *testkit.Client, path string) OAuthAttempt {
	tc.T.Helper()

	res := client.Send(fhttp.MethodGet, path, nil).AssertStatus(fhttp.StatusFound)

	return OAuthAttempt{AuthURL: res.Header.Get("Location"), Cookie: res.Cookie(tc.App.Cookies.OAuthName())}
}

/*
FinishOAuth approves the attempt at the fake provider as the person and
follows the callback with the client and the attempt's cookie.
*/
func (tc *TestCase) FinishOAuth(client *testkit.Client, attempt OAuthAttempt, person socialite.User) *testkit.Response {
	tc.T.Helper()

	callback, err := tc.Social().Approve(attempt.AuthURL, person)
	require.NoError(tc.T, err)

	return client.WithCookie(attempt.Cookie).Send(fhttp.MethodGet, callback, nil)
}

/*
SignInWith signs a fresh browser in with the provider as the person
and returns the callback's response.
*/
func (tc *TestCase) SignInWith(provider string, person socialite.User) *testkit.Response {
	tc.T.Helper()

	tc.Social()
	attempt := tc.StartOAuth(tc.Client(), "/auth/"+provider+"/redirect")

	return tc.FinishOAuth(tc.Client(), attempt, person)
}
