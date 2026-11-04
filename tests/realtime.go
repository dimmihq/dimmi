/*
|--------------------------------------------------------------------------
| Realtime
|--------------------------------------------------------------------------
|
| Realtime tests connect the way the dashboard does, to a real realtime
| endpoint on a real port: a browser opens the socket from the app's own
| page, carrying its session cookie, while apps and integrations send
| their token in the connect command. Nothing about delivery is faked:
| a broadcast a test sees went through Valkey like any other.
|
*/

package tests

import (
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
RealtimeURL serves the application's realtime endpoint on a local port
until the test ends and returns its base URL.
*/
func (tc *TestCase) RealtimeURL() string {
	tc.T.Helper()

	return testkit.Serve(tc.T, tc.App.Broadcast.Handlers()[broadcasting.PathWebsocket])
}

/*
BrowserHeaders returns the headers a browser sends when the user's
signed-in page at the base URL opens a realtime connection: the session
cookie and the page's own origin.
*/
func (tc *TestCase) BrowserHeaders(user models.User, baseURL string) fhttp.Header {
	tc.T.Helper()

	res := tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "password", "email": user.Email, "password": factories.Password,
	}).AssertCreated()

	cookie := res.Cookie(tc.App.Session.CookieName())
	require.NotNil(tc.T, cookie)

	return fhttp.Header{"Cookie": {cookie.Name + "=" + cookie.Value}, "Origin": {baseURL}}
}

/*
ListenAs connects to the realtime endpoint at the base URL as the
user's browser would, failing the test unless the connection opens.
*/
func (tc *TestCase) ListenAs(user models.User, baseURL string) *testkit.Realtime {
	tc.T.Helper()

	return testkit.ConnectRealtime(tc.T, baseURL, testkit.RealtimeOptions{Header: tc.BrowserHeaders(user, baseURL)})
}

/*
ListenWithToken connects to the realtime endpoint at the base URL with
a fresh token of the member holding the abilities.
*/
func (tc *TestCase) ListenWithToken(
	member models.WorkspaceMember, baseURL string, abilities ...string,
) *testkit.Realtime {
	tc.T.Helper()

	_, plain := tc.MintToken(member, abilities...)

	return testkit.ConnectRealtime(tc.T, baseURL, testkit.RealtimeOptions{Token: plain})
}
