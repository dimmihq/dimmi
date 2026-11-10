package integrations_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
slackCase is a test case with the shared Slack app configured and every
provider faked, and the owner signed in.
*/
type slackCase struct {
	tc    *tests.TestCase
	fake  *oauthclient.Fake
	owner *testkit.Client
}

/*
newSlackCase boots the application with the shared Slack app.
*/
func newSlackCase(t *testing.T) slackCase {
	t.Helper()

	tc := tests.New(t, tests.WithEnv(
		"DIMMI_SERVICES__SLACK__CLIENT_ID=app-id", "DIMMI_SERVICES__SLACK__CLIENT_SECRET=app-secret",
		"DIMMI_SERVICES__SLACK__SIGNING_SECRET=app-signing",
	))

	return slackCase{tc: tc, fake: tc.App.OAuthClient.Fake(), owner: tc.In(tc.Owner)}
}

/*
allScopes is every bot scope the Slack features registered.
*/
func (s slackCase) allScopes() []string {
	return s.tc.App.Actions.Integrations.Registry.BotScopes("slack", nil)
}

/*
slackGrant is what Slack returns for a team's install.
*/
func slackGrant(team string, scopes []string) oauthclient.Grant {
	return oauthclient.Grant{
		Access: &oauthclient.Token{Value: "xoxb-" + team + "-" + ids.New().String()}, Scopes: scopes,
		Account: team, AccountName: "Team " + team, App: "A1",
		Raw: map[string]any{"bot_user_id": "UB1", "is_enterprise_install": false},
	}
}

/*
start asks for an install as the client and returns the authorize URL.
*/
func start(t *testing.T, client *testkit.Client, body testkit.JSON) string {
	t.Helper()

	res := client.PostJSON("/api/v1/integrations/slack/installations", body).AssertOK()

	return res.String("data.authorize_url")
}

/*
follow opens the callback URL as the client and returns where it sends
the browser.
*/
func follow(t *testing.T, client *testkit.Client, callback string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(callback)
	require.NoError(t, err)

	res := client.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusFound)

	location, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)

	return location
}

/*
install runs a whole install as the owner, approved with the grant, and
returns the installation's public id.
*/
func (s slackCase) install(t *testing.T, grant oauthclient.Grant) string {
	t.Helper()

	return s.installFor(t, grant, testkit.JSON{})
}

/*
quietInstall installs for notifications only, so no channel connection
starts syncing with the installation's token behind the test's back.
*/
func (s slackCase) quietInstall(t *testing.T, grant oauthclient.Grant) string {
	t.Helper()

	return s.installFor(t, grant, testkit.JSON{"features": []string{"notifications"}})
}

/*
installFor runs a whole install with the request body.
*/
func (s slackCase) installFor(t *testing.T, grant oauthclient.Grant, body testkit.JSON) string {
	t.Helper()

	callback, err := s.fake.Approve(start(t, s.owner, body), grant)
	require.NoError(t, err)

	landing := follow(t, s.owner, callback)
	require.Empty(t, landing.Query().Get("error"))
	require.NotEmpty(t, landing.Query().Get("connected"))

	return landing.Query().Get("connected")
}

/*
id decodes an installation's public id.
*/
func (s slackCase) id(t *testing.T, public string) ids.UUID {
	t.Helper()

	id, err := s.tc.App.IDs.Decode(string(models.PrefixIntegrationInstallation), public)
	require.NoError(t, err)

	return id
}

/*
join adds a member with the role and returns their client and row.
*/
func (s slackCase) join(t *testing.T, role enums.Role) (*testkit.Client, models.WorkspaceMember) {
	t.Helper()

	user := s.tc.CreateUser()
	member, err := factories.Member(t.Context(), s.tc.App, factories.Membership{
		Workspace: s.tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return s.tc.In(user), member
}

/*
scoped returns a context in the test case's workspace.
*/
func (s slackCase) scoped(t *testing.T) context.Context {
	t.Helper()

	return tenancy.With(t.Context(), s.tc.Workspace.Workspace.ID.String())
}

/*
grantAs runs a member's grant on the installation as the client,
approved with the grant, and returns where the browser landed.
*/
func (s slackCase) grantAs(t *testing.T, client *testkit.Client, public string, grant oauthclient.Grant) *url.URL {
	t.Helper()

	res := client.PostJSON("/api/v1/integrations/"+public+"/member-grants", testkit.JSON{}).AssertOK()

	callback, err := s.fake.Approve(res.String("data.authorize_url"), grant)
	require.NoError(t, err)

	return follow(t, client, callback)
}

/*
userGrant is what Slack returns for a teammate's own grant.
*/
func userGrant(team, user string) oauthclient.Grant {
	return oauthclient.Grant{
		UserAccess: &oauthclient.Token{Value: "xoxp-" + user + "-" + ids.New().String()}, UserID: user,
		UserScopes: []string{"chat:write", "reactions:write"}, Account: team,
	}
}

/*
expiring is a grant whose app token expires in a minute and can be
renewed with the refresh token.
*/
func (s slackCase) expiring(refresh string) oauthclient.Grant {
	grant := slackGrant("T1", s.allScopes())
	grant.Access.Expiry = s.tc.Clock.Now().Add(time.Minute)
	grant.Refresh = &oauthclient.Token{Value: refresh}

	return grant
}

/*
manual is the owner pasting a custom app's bot token for Slack.
*/
func (s slackCase) manual(token string) integrations.ManualInstallationInput {
	return integrations.ManualInstallationInput{Membership: s.ownerMembership(), Provider: "slack", Token: token}
}

/*
hasToken reports whether the text carries a Slack token.
*/
func hasToken(text string) bool {
	return strings.Contains(text, "xoxb-") || strings.Contains(text, "xoxp-")
}
