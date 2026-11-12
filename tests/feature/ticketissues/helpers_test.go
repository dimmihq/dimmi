package ticketissues_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/integrations/trackers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
	githubfixtures "github.com/dimmihq/dimmi/tests/fixtures/github"
	incidentiofixtures "github.com/dimmihq/dimmi/tests/fixtures/incident_io"
	linearfixtures "github.com/dimmihq/dimmi/tests/fixtures/linear"
)

/*
The secrets the apps' webhooks are signed with in tests.
*/
const (
	linearSecret   = "lin_wh_test"
	githubSecret   = "gh_wh_test"                             //nolint:gosec // a test value the fake signs with
	incidentSecret = "whsec_dGVzdC1zZWNyZXQtZm9yLWluY2lkZW50" //nolint:gosec // a test value the fake signs with
	jiraSecret     = "jira_client_secret"                     //nolint:gosec // a test value the fake signs with
	toolSecret     = "tool_signing_secret"                    //nolint:gosec // a test value the fake signs with
)

/*
desk is a workspace with every engineering tool faked from recorded
answers.
*/
type desk struct {
	tests.Desk

	fake   *httpclienttest.Fake
	types  map[string]string
	mu     *sync.Mutex
	linear *[]string
	hidden *bool
}

/*
newDesk boots the application with the tools' webhook secrets and
fakes Linear, incident.io and GitHub.
*/
func newDesk(t *testing.T) desk {
	t.Helper()

	tc := tests.New(t, tests.WithEnv(
		"DIMMI_SERVICES__LINEAR__WEBHOOK_SECRET="+linearSecret,
		"DIMMI_SERVICES__TRACKERS__GITHUB_WEBHOOK_SECRET="+githubSecret,
		"DIMMI_SERVICES__JIRA__CLIENT_ID=jira-app", "DIMMI_SERVICES__JIRA__CLIENT_SECRET="+jiraSecret,
	))
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	hidden := false
	d := desk{
		Desk: tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)},
		fake: httpclienttest.New(tc.App.HTTP).PreventStrayRequests(), types: map[string]string{},
		mu: &sync.Mutex{}, linear: &[]string{}, hidden: &hidden,
	}

	res := d.Owner.GetJSON("/api/v1/ticket-types").AssertOK()
	for i := range count(res) {
		d.types[res.String("data."+strconv.Itoa(i)+".name")] = res.String("data." + strconv.Itoa(i) + ".id")
	}

	d.serveLinear(t)
	d.serveIncidentIO(t)
	d.serveGitHub(t)
	d.serveJira(t)
	d.serveKeyedTools(t)

	return d
}

/*
count reads how many items a response's data holds.
*/
func count(res *testkit.Response) int {
	n, _ := strconv.Atoi(res.String("data.#"))

	return n
}

/*
fixture reads a recorded answer.
*/
func fixture(t *testing.T, files fs.FS, name string) []byte {
	t.Helper()

	body, err := fs.ReadFile(files, name)
	require.NoError(t, err)

	return body
}

/*
reply is a JSON answer.
*/
func reply(body []byte) httpclient.Response {
	return httpclient.Response{Status: 200, Header: fhttp.Header{"Content-Type": {"application/json"}}, Body: body}
}

/*
serveLinear answers Linear's GraphQL endpoint by the operation each
request runs, and keeps every query sent.
*/
func (d desk) serveLinear(t *testing.T) {
	t.Helper()

	d.fake.Serve("POST https://api.linear.app/graphql", fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		raw, _ := io.ReadAll(r.Body)

		d.mu.Lock()
		*d.linear = append(*d.linear, string(raw))
		hidden := *d.hidden
		d.mu.Unlock()

		name := "ok.json"

		switch q := string(raw); {
		case strings.Contains(q, "organization"):
			name = "organization.json"
		case strings.Contains(q, "teams(first"):
			name = "teams.json"
		case strings.Contains(q, "states {"):
			name = "states.json"
		case strings.Contains(q, "issueCreate"):
			name = "issue_create.json"
		case strings.Contains(q, "issue(id") && hidden:
			name = "issue_hidden.json"
		case strings.Contains(q, "issue(id"):
			name = "issue.json"
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, linearfixtures.Files, name))
	}))
}

/*
linearSent returns the GraphQL requests sent so far.
*/
func (d desk) linearSent() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]string(nil), *d.linear...)
}

/*
serveIncidentIO answers incident.io's identity and severities.
*/
func (d desk) serveIncidentIO(t *testing.T) {
	t.Helper()

	d.fake.Stub("GET https://api.incident.io/v1/identity", reply(fixture(t, incidentiofixtures.Files, "identity.json")))
	d.fake.Stub("GET https://api.incident.io/v1/severities",
		reply(fixture(t, incidentiofixtures.Files, "severities.json")))
}

/*
serveGitHub answers the GitHub account, issue acme/app#88 and a new
repository webhook.
*/
func (d desk) serveGitHub(t *testing.T) {
	t.Helper()

	d.fake.Stub("GET https://api.github.com/user", reply(fixture(t, githubfixtures.Files, "user.json")))
	d.fake.Stub("GET https://api.github.com/repos/acme/app/issues/88",
		reply(fixture(t, githubfixtures.Files, "issue.json")))
	d.fake.Stub("POST https://api.github.com/repos/acme/app/hooks", reply([]byte(`{"id": 1}`)))
	d.fake.Stub("GET https://api.github.com/repos/acme/app/issues/88/comments?per_page=100", reply([]byte(`[]`)))
	d.fake.Stub("POST https://api.github.com/repos/acme/app/issues/88/comments", reply([]byte(`{"id": 2}`)))
}

/*
connect installs the tool from a token, as an OAuth install would, and
returns the installation's public id.
*/
func (d desk) connect(t *testing.T, provider, token string) string {
	t.Helper()

	ws := d.TC.Workspace
	owner := authz.NewMembership(ws.Workspace, ws.Owner, auth.RolePermissions(enums.RoleOwner))
	row, err := d.TC.App.Actions.Integrations.CreateManual.Handle(t.Context(), integrations.ManualInstallationInput{
		Membership: owner, Provider: provider, Token: token,
	})
	require.NoError(t, err)

	return models.PublicID(models.PrefixIntegrationInstallation, row.ID)
}

/*
tracker makes a tracker linked to a conversation of each customer.
*/
func (d desk) tracker(t *testing.T, title string, customers ...string) string {
	t.Helper()

	id := d.Owner.PostJSON("/api/v1/tickets", testkit.JSON{"type_id": d.types["Tracker"], "title": title}).
		AssertCreated().String("data.id")

	for _, name := range customers {
		conv := d.Customer(t, name, title).String("data.id")
		d.Owner.PostJSON("/api/v1/tickets/"+id+"/links", testkit.JSON{"kind": "tracker", "conversation_id": conv}).
			AssertOK()
	}

	return id
}

/*
linearDelivery signs a recorded Linear delivery stamped now.
*/
func (d desk) linearDelivery(t *testing.T, name string) *testkit.Response {
	t.Helper()

	var payload map[string]any
	require.NoError(t, json.Unmarshal(fixture(t, linearfixtures.Files, name), &payload))
	payload["webhookTimestamp"] = d.TC.Clock.Now().UnixMilli()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	return d.TC.Client().WithHeader("Linear-Signature", trackers.Sign(linearSecret, body)).
		Send(fhttp.MethodPost, "/webhooks/linear", body)
}

/*
incidentDelivery signs a recorded incident.io delivery to the
installation's address.
*/
func (d desk) incidentDelivery(t *testing.T, installation, name string) *testkit.Response {
	t.Helper()

	body := fixture(t, incidentiofixtures.Files, name)
	ts := strconv.FormatInt(d.TC.Clock.Now().Unix(), 10)

	return d.TC.Client().WithHeader("Webhook-Id", "msg_"+name).WithHeader("Webhook-Timestamp", ts).
		WithHeader("Webhook-Signature", trackers.SvixSignature(incidentSecret, "msg_"+name, ts, body)).
		Send(fhttp.MethodPost, "/webhooks/incident-io/"+installation, body)
}

/*
githubDelivery signs a recorded GitHub delivery to the installation's
address.
*/
func (d desk) githubDelivery(t *testing.T, installation, name string) *testkit.Response {
	t.Helper()

	body := fixture(t, githubfixtures.Files, name)

	return d.TC.Client().WithHeader("X-Hub-Signature-256", "sha256="+trackers.Sign(githubSecret, body)).
		Send(fhttp.MethodPost, "/webhooks/github-issues?installation="+installation, bytes.Clone(body))
}
