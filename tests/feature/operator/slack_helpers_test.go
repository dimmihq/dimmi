package operator_test

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/drivers/llm/fake"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
The Slack workspace Operator is asked in: its team, the app, the bot,
the team's own room and the owner's Slack user.
*/
const (
	slackTeam  = "T0ACME"
	slackBot   = "U0BOT"
	teamRoom   = "C0TEAM"
	ownerSlack = "U0OWNER"
	askedTS    = "1793000100.000100"
	placedTS   = "1793000500.000100"
)

/*
slackCall is one Web API call the fake Slack received.
*/
type slackCall struct {
	Method string
	Form   url.Values
}

/*
fakeSlack answers the Web API calls and records them.
*/
type fakeSlack struct {
	mu    sync.Mutex
	calls []slackCall
}

/*
serve answers one call as Slack does when it goes well.
*/
func (f *fakeSlack) serve(w fhttp.ResponseWriter, r *fhttp.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	_ = r.ParseForm()

	f.mu.Lock()
	f.calls = append(f.calls, slackCall{Method: method, Form: r.Form})
	f.mu.Unlock()

	body := `{"ok": true}`

	switch method {
	case "users.conversations":
		body = `{"ok": true, "channels": [{"id": "` + teamRoom + `", "name": "team"}],
			"response_metadata": {"next_cursor": ""}}`
	case "chat.postMessage":
		body = `{"ok": true, "channel": "` + r.Form.Get("channel") + `", "ts": "` + placedTS + `"}`
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

/*
called returns the calls of the method, oldest first.
*/
func (f *fakeSlack) called(method string) []slackCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []slackCall

	for _, c := range f.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}

	return out
}

/*
slackDesk is an Operator desk with Slack installed and faked, and the
connection the installation made.
*/
type slackDesk struct {
	*desk
	slack *fakeSlack
	conn  models.ChannelConnection
}

/*
newSlackDesk boots a desk whose model is scripted and whose Slack is
installed for channels and Operator.
*/
func newSlackDesk(t *testing.T) *slackDesk {
	t.Helper()

	tc := tests.New(t, tests.WithEnv(
		"DIMMI_SERVICES__SLACK__CLIENT_ID=app-id", "DIMMI_SERVICES__SLACK__CLIENT_SECRET=app-secret",
	))
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	d := tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
	k := &desk{t: t, d: d, llm: fake.New(contracts.LLMProviderAnthropic)}
	tc.App.LLM.Extend(enums.AIProvider("fake"), func(
		_ context.Context, _ *managers.LLMManager, _ models.AiProviderKey, _ managers.Credentials, _ *httpclient.Client,
	) (contracts.LLM, error) {
		return k.llm, nil
	})
	tc.InWorkspace(func(ctx context.Context) error { return connect(ctx, t, d) })

	s := &slackDesk{desk: k, slack: &fakeSlack{}}
	httpclienttest.New(tc.App.HTTP).Serve("https://slack.com/api/*", fhttp.HandlerFunc(s.slack.serve))
	s.conn = s.install()

	return s
}

/*
install connects Slack for channels and Operator and returns the
connection it made.
*/
func (s *slackDesk) install() models.ChannelConnection {
	s.t.Helper()

	tc := s.d.TC
	oauth := tc.App.OAuthClient.Fake()
	res := s.d.Owner.PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{
		"features": []string{"channels", "operator"},
	}).AssertOK()

	callback, err := oauth.Approve(res.String("data.authorize_url"), oauthclient.Grant{
		Access:  &oauthclient.Token{Value: "xoxb-" + ids.New().String()},
		Scopes:  tc.App.Actions.Integrations.Registry.BotScopes("slack", nil),
		Account: slackTeam, AccountName: "Acme", App: "A0APP",
		Raw: map[string]any{"bot_user_id": slackBot, "is_enterprise_install": false},
	})
	require.NoError(s.t, err)

	parsed, err := url.Parse(callback)
	require.NoError(s.t, err)

	landing := s.d.Owner.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusFound)
	location, err := url.Parse(landing.Header.Get("Location"))
	require.NoError(s.t, err)

	installation, err := tc.App.IDs.Decode(string(models.PrefixIntegrationInstallation),
		location.Query().Get("connected"))
	require.NoError(s.t, err)
	tc.Drain()

	var conn models.ChannelConnection

	require.NoError(s.t, tc.App.DB.Transaction(s.scoped(), func(ctx context.Context, tx *database.Tx) error {
		params := models.FindInstallationChannelConnectionParams{
			WorkspaceID: tc.Workspace.Workspace.ID, IntegrationInstallationID: &installation,
		}

		var err error
		conn, err = models.New(tx).FindInstallationChannelConnection(ctx, params)

		return err
	}))

	return conn
}

/*
scoped is a context in the desk's workspace.
*/
func (s *slackDesk) scoped() context.Context {
	return tenancy.With(s.t.Context(), s.d.TC.Workspace.Workspace.ID.String())
}

/*
roomMode sets how the team's room is watched.
*/
func (s *slackDesk) roomMode(mode string) {
	s.t.Helper()

	_, err := s.d.TC.Operator().Exec(s.t.Context(),
		"UPDATE channel_rooms SET mode = $1 WHERE channel_connection_id = $2 AND external_id = $3",
		mode, s.conn.ID, teamRoom)
	require.NoError(s.t, err)
}

/*
link links the owner's Slack user to the owner.
*/
func (s *slackDesk) link() {
	s.t.Helper()

	_, err := s.d.TC.Operator().Exec(s.t.Context(),
		"INSERT INTO channel_member_links (workspace_id, channel_connection_id, workspace_member_id, "+
			"external_user_id, source) VALUES ($1, $2, $3, $4, 'manual')",
		s.conn.WorkspaceID, s.conn.ID, s.d.TC.Workspace.Owner.ID, slackdriver.UserIdentity(slackTeam, ownerSlack))
	require.NoError(s.t, err)
}

/*
deliver hands the event to the router and reports whether a feature
took it.
*/
func (s *slackDesk) deliver(event map[string]any) bool {
	s.t.Helper()

	raw, err := json.Marshal(event)
	require.NoError(s.t, err)

	ev, err := slackdriver.ParseEvent(raw)
	require.NoError(s.t, err)

	handled, err := s.d.TC.App.Actions.Slack.Router.DispatchEvent(s.scoped(), channels.SlackEvent{
		Connection: s.conn, Event: ev,
	})
	require.NoError(s.t, err)

	return handled
}

/*
mention is the owner mentioning the app in the room, in a thread when
one is named.
*/
func mention(user, text, thread string) map[string]any {
	ev := map[string]any{
		"type": "app_mention", "channel": teamRoom, "user": user, "team": slackTeam, "ts": askedTS,
		"text": "<@" + slackBot + "> " + text,
	}
	if thread != "" {
		ev["thread_ts"], ev["ts"] = thread, "1793000200.000100"
	}

	return ev
}

/*
press has the Slack user press a button with the value.
*/
func (s *slackDesk) press(user, action, value string) {
	s.t.Helper()

	raw, err := json.Marshal(map[string]any{
		"type": "block_actions", "trigger_id": ids.New().String(),
		"user": map[string]any{"id": user, "team_id": slackTeam}, "team": map[string]any{"id": slackTeam},
		"channel": map[string]any{"id": teamRoom},
		"container": map[string]any{
			"type": "message", "channel_id": teamRoom, "message_ts": placedTS, "thread_ts": askedTS,
		},
		"actions": []map[string]any{{"action_id": action, "value": value}},
	})
	require.NoError(s.t, err)

	in, err := slackdriver.ParseInteraction(raw)
	require.NoError(s.t, err)

	handled, err := s.d.TC.App.Actions.Slack.Router.DispatchInteraction(s.scoped(), channels.SlackInteraction{
		Connection: s.conn, Interaction: in,
	})
	require.NoError(s.t, err)
	require.True(s.t, handled)
}
