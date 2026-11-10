package oauth_test

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/hashing"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mcp"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
callback is where the test app receives its answer.
*/
const callback = "http://127.0.0.1:43210/callback"

/*
verifier is the test app's PKCE secret.
*/
const verifier = "the-test-apps-code-verifier-which-is-long-enough-1234567890"

/*
app is a test app that registered itself, as MCP clients do.
*/
type app struct {
	tc *tests.TestCase
	id string
}

/*
register registers a test app through dynamic client registration.
*/
func register(t *testing.T, tc *tests.TestCase) app {
	t.Helper()
	res := tc.Client().PostJSON("/oauth/register", testkit.JSON{
		"client_name": "Test Agent", "redirect_uris": []string{callback}, "token_endpoint_auth_method": "none",
		"grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"},
	}).AssertCreated()

	return app{tc: tc, id: res.String("client_id")}
}

/*
query is the app's authorization request for the scopes.
*/
func (a app) query(scopes string) string {
	sum := hashing.SHA256([]byte(verifier))

	return url.Values{
		"response_type": {"code"}, "client_id": {a.id}, "redirect_uri": {callback}, "scope": {scopes},
		"state": {"xyz"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum)},
		"code_challenge_method": {"S256"},
	}.Encode()
}

/*
form posts a form to the OAuth server.
*/
func (a app) form(path string, values url.Values) *testkit.Response {
	return a.tc.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
		Send(fhttp.MethodPost, path, values.Encode())
}

/*
connect walks the whole flow as the owner, approving the scopes in the
test's workspace, and returns the tokens.
*/
func (a app) connect(t *testing.T, scopes string) (string, string) {
	t.Helper()

	return a.connectAs(t, a.tc.ActingAs(a.tc.Owner), scopes)
}

/*
connectAs walks the whole flow as the person behind the client.
*/
func (a app) connectAs(t *testing.T, person *testkit.Client, scopes string) (string, string) {
	t.Helper()
	workspace := models.PublicID(models.PrefixWorkspace, a.tc.Workspace.Workspace.ID)

	to := person.PostJSON("/api/v1/oauth/authorizations", testkit.JSON{
		"query": a.query(scopes), "decision": "approve", "workspace_id": workspace,
	}).AssertOK().String("data.redirect_to")

	back, err := url.Parse(to)
	require.NoError(t, err)
	require.Equal(t, "xyz", back.Query().Get("state"))

	res := a.form("/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {back.Query().Get("code")}, "redirect_uri": {callback},
		"client_id": {a.id}, "code_verifier": {verifier},
	}).AssertOK()

	return res.String("access_token"), res.String("refresh_token")
}

func TestConnectingAnApp(t *testing.T) {
	t.Parallel()

	t.Run("shows the person what the app asks for and where they may connect it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		a := register(t, tc)

		res := tc.ActingAs(tc.Owner).GetJSON("/api/v1/oauth/authorization?query=" +
			url.QueryEscape(a.query("conversations:read offline_access"))).AssertOK()

		assert.Equal(t, "Test Agent", res.String("data.client_name"))
		assert.Equal(t, "true", res.String("data.self_registered"))
		assert.Equal(t, "true", res.String("data.loopback"))
		assert.Equal(t, "conversations:read", res.String("data.scopes.0"))
		assert.Empty(t, res.String("data.scopes.1"))
		assert.Equal(t, tc.Workspace.Workspace.Name, res.String("data.workspaces.0.name"))
	})

	t.Run("lets the connected app act as the person with the access they gave", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		access, _ := register(t, tc).connect(t, "conversations:read offline_access")
		agent := tc.Client().WithHeader("Authorization", "Bearer "+access)

		agent.GetJSON("/api/v1/conversations").AssertOK()
		agent.PostJSON("/api/v1/tags", testkit.JSON{"name": "vip"}).AssertForbidden()
	})

	t.Run("signs the app in to the MCP server with its token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		access, _ := register(t, tc).connect(t, "conversations:read")
		hc, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
		require.NoError(t, err)

		client, err := mcp.Connect(t.Context(), tc.URL()+"/mcp", mcp.ClientOptions{
			Name: "agent", Version: "1", HTTP: hc, Headers: fhttp.Header{"Authorization": {"Bearer " + access}},
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = client.Close() })

		res, err := client.CallTool(t.Context(), "dimmi_conversations_list", map[string]any{})
		require.NoError(t, err)
		assert.False(t, res.IsError, res.Text)
	})

	t.Run("renews access with the refresh token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		a := register(t, tc)
		_, refresh := a.connect(t, "conversations:read offline_access")

		res := a.form("/oauth/token", url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {a.id},
		}).AssertOK()

		tc.Client().WithHeader("Authorization", "Bearer "+res.String("access_token")).
			GetJSON("/api/v1/conversations").AssertOK()
	})

	t.Run("sends a refusal back to the app", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		a := register(t, tc)

		to := tc.ActingAs(tc.Owner).PostJSON("/api/v1/oauth/authorizations", testkit.JSON{
			"query": a.query("conversations:read"), "decision": "deny",
		}).AssertOK().String("data.redirect_to")

		assert.True(t, strings.HasPrefix(to, callback+"?"), to)
		assert.Contains(t, to, "error=access_denied")
	})

	t.Run("refuses a workspace the person is not in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		a := register(t, tc)
		stranger := tc.CreateUser()

		tc.ActingAs(stranger).PostJSON("/api/v1/oauth/authorizations", testkit.JSON{
			"query": a.query("conversations:read"), "decision": "approve",
			"workspace_id": models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID),
		}).AssertValidationError("workspace_id")
	})

	t.Run("refuses an unknown app without sending the person anywhere", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		query := app{tc: tc, id: "dcr_unknown"}.query("conversations:read")

		res := tc.ActingAs(tc.Owner).GetJSON("/api/v1/oauth/authorization?query=" + url.QueryEscape(query))

		res.AssertStatus(422)
		assert.Empty(t, res.String("redirect_to"))
	})

	t.Run("never lets the app do more than the person's role allows", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		member, _ := d.Join(t, enums.RoleMember)
		access, _ := register(t, d.TC).connectAs(t, member, "settings:read")

		d.TC.Client().WithHeader("Authorization", "Bearer "+access).GetJSON("/api/v1/redaction-rules").
			AssertForbidden()
	})
}

func TestConnectedApps(t *testing.T) {
	t.Parallel()

	t.Run("lists the person's apps and disconnects one at once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		access, _ := register(t, tc).connect(t, "conversations:read")
		owner := tc.ActingAs(tc.Owner)

		listed := owner.GetJSON("/api/v1/user/connected-apps").AssertOK()
		assert.Equal(t, "Test Agent", listed.String("data.0.name"))
		assert.Equal(t, tc.Workspace.Workspace.Name, listed.String("data.0.workspace_name"))

		owner.Delete("/api/v1/user/connected-apps/" + listed.String("data.0.id")).AssertNoContent()

		tc.Client().WithHeader("Authorization", "Bearer "+access).GetJSON("/api/v1/conversations").
			AssertUnauthorized()
		assert.Empty(t, owner.GetJSON("/api/v1/user/connected-apps").AssertOK().String("data.0.id"))
	})

	t.Run("hides other people's apps", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		register(t, tc).connect(t, "conversations:read")
		id := tc.ActingAs(tc.Owner).GetJSON("/api/v1/user/connected-apps").AssertOK().String("data.0.id")

		tc.ActingAs(tc.CreateUser()).Delete("/api/v1/user/connected-apps/" + id).AssertNotFound()
	})
}

func TestDiscovery(t *testing.T) {
	t.Parallel()

	t.Run("points an MCP client that finds /mcp locked to the sign-in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().PostJSON("/mcp", testkit.JSON{}).AssertUnauthorized()
		res.AssertHeader("WWW-Authenticate",
			`Bearer resource_metadata="`+tc.App.Application.Config.App.URL+`/.well-known/oauth-protected-resource/mcp"`)

		meta := tc.Client().GetJSON("/.well-known/oauth-protected-resource/mcp").AssertOK()
		assert.Equal(t, tc.App.Application.Config.App.URL, meta.String("authorization_servers.0"))

		server := tc.Client().GetJSON("/.well-known/oauth-authorization-server").AssertOK()
		assert.NotEmpty(t, server.String("registration_endpoint"))
		assert.Contains(t, server.String("authorization_endpoint"), "/oauth/authorize")
	})
}
