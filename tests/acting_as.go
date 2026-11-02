/*
|--------------------------------------------------------------------------
| Acting As
|--------------------------------------------------------------------------
|
| Tests act as someone the way a real client does, so every request goes
| through the real guards. A browser user signs in with their password
| through the API and the client keeps the cookies the response set; a
| token holder gets a real token minted for their membership.
|
*/

package tests

import (
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

/*
ActingAs returns a client signed in as the user with a browser session.
The user must have the factory password.
*/
func (tc *TestCase) ActingAs(user models.User) *testkit.Client {
	tc.T.Helper()

	res := tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "password", "email": user.Email, "password": factories.Password,
	}).AssertCreated()

	return tc.Client().
		WithCookie(res.Cookie(tc.App.Session.CookieName())).
		WithCookie(res.Cookie(tc.App.Cookies.DeviceName()))
}

/*
ActingAsOwner returns a client signed in as the test case's workspace
owner.
*/
func (tc *TestCase) ActingAsOwner() *testkit.Client {
	tc.T.Helper()

	return tc.ActingAs(tc.Owner)
}

/*
ActingAsToken returns a client sending a fresh personal access token of
the member, bound to their workspace, with the abilities.
*/
func (tc *TestCase) ActingAsToken(member models.WorkspaceMember, abilities ...string) *testkit.Client {
	tc.T.Helper()

	_, plain := tc.MintToken(member, abilities...)

	return tc.Client().WithHeader("Authorization", "Bearer "+plain)
}

/*
MintToken creates a personal access token for the member and returns it
with its plain value.
*/
func (tc *TestCase) MintToken(member models.WorkspaceMember, abilities ...string) (token.Token, string) {
	tc.T.Helper()

	tok, plain, err := tc.App.Tokens.Live.Create(tc.T.Context(), token.Token{
		OwnerID: member.ID, TenantID: member.WorkspaceID.String(), Name: "Test token", Abilities: abilities,
	})
	require.NoError(tc.T, err)

	return tok, plain
}

/*
CreateUser creates an account in this test's application, changed by
the factory states.
*/
func (tc *TestCase) CreateUser(states ...func(models.User) models.User) models.User {
	tc.T.Helper()

	users, err := factories.User(tc.T.Context(), tc.App)
	require.NoError(tc.T, err)

	for _, state := range states {
		users = users.State(state)
	}

	user, err := users.CreateOne(tc.T.Context())
	require.NoError(tc.T, err)

	return user
}
