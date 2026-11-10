package notifications_test

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
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
The Slack workspace the tests install into: its team, the app, the bot
and the channel alerts post in.
*/
const (
	slackTeam    = "T0ACME"
	slackApp     = "A0APP"
	slackBotUser = "U0BOT"
	supportRoom  = "C0SUPPORT"
	ownerSlack   = "U0OWNER"
	ownerDM      = "D0OWNER"
)

/*
slackCall is one Web API call the fake Slack received.
*/
type slackCall struct {
	Method string
	Form   url.Values
}

/*
fakeSlack answers the Web API calls notifications make and records
them. Answers scripted for a method win over the default ones.
*/
type fakeSlack struct {
	mu      sync.Mutex
	calls   []slackCall
	answers map[string][]string
}

/*
serve answers one call.
*/
func (f *fakeSlack) serve(w fhttp.ResponseWriter, r *fhttp.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	_ = r.ParseForm()

	f.mu.Lock()
	f.calls = append(f.calls, slackCall{Method: method, Form: r.Form})

	body, scripted := "", false
	if queue := f.answers[method]; len(queue) > 0 {
		body, scripted, f.answers[method] = queue[0], true, queue[1:]
	}
	f.mu.Unlock()

	if !scripted {
		body = defaultAnswer(method, r.Form)
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

/*
defaultAnswer is what Slack answers a call that went well.
*/
func defaultAnswer(method string, form url.Values) string {
	switch method {
	case "auth.test":
		return `{"ok": true, "team_id": "` + slackTeam + `", "team": "Acme", "user_id": "` + slackBotUser +
			`", "bot_id": "B0BOT"}`
	case "conversations.info":
		return `{"ok": true, "channel": {"id": "` + form.Get("channel") + `", "name": "support"}}`
	case "users.conversations":
		return `{"ok": true, "channels": [{"id": "` + supportRoom + `", "name": "support"}],
			"response_metadata": {"next_cursor": ""}}`
	case "users.lookupByEmail":
		return `{"ok": true, "user": {"id": "` + ownerSlack + `", "team_id": "` + slackTeam + `", "name": "owner"}}`
	case "conversations.open":
		return `{"ok": true, "channel": {"id": "` + ownerDM + `"}}`
	case "chat.postMessage":
		return `{"ok": true, "channel": "` + form.Get("channel") + `", "ts": "1793000500.000100"}`
	}

	return `{"ok": true}`
}

/*
answer scripts the next answers of the method.
*/
func (f *fakeSlack) answer(method string, bodies ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.answers[method] = append(f.answers[method], bodies...)
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
slackDesk is a desk with the shared Slack app configured and Slack
faked.
*/
type slackDesk struct {
	*desk
	slack *fakeSlack
	oauth *oauthclient.Fake
}

/*
newSlackDesk boots a desk whose Slack is faked.
*/
func newSlackDesk(t *testing.T) *slackDesk {
	t.Helper()

	tc := tests.New(t, tests.WithEnv(
		"DIMMI_SERVICES__SLACK__CLIENT_ID=app-id", "DIMMI_SERVICES__SLACK__CLIENT_SECRET=app-secret",
	))
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	d := &desk{Desk: tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}, t: t}
	d.mail = tc.Mail()
	tc.InWorkspace(func(ctx context.Context) error {
		rows, err := models.New(tc.App.DB).ListWidgetSettings(ctx, tc.Workspace.Workspace.ID)
		require.NoError(t, err)
		require.NotEmpty(t, rows)
		d.app = models.PublicID(models.PrefixWidget, rows[0].ID)

		return nil
	})

	fake := &fakeSlack{answers: map[string][]string{}}
	httpclienttest.New(tc.App.HTTP).Serve("https://slack.com/api/*", fhttp.HandlerFunc(fake.serve))

	return &slackDesk{desk: d, slack: fake, oauth: tc.App.OAuthClient.Fake()}
}

/*
install connects Slack for the features as the owner and returns the
installation's public id.
*/
func (s *slackDesk) install(features ...string) string {
	s.t.Helper()

	res := s.Owner.PostJSON("/api/v1/integrations/slack/installations", testkit.JSON{"features": features}).AssertOK()

	callback, err := s.oauth.Approve(res.String("data.authorize_url"), oauthclient.Grant{
		Access:  &oauthclient.Token{Value: "xoxb-" + ids.New().String()},
		Scopes:  s.TC.App.Actions.Integrations.Registry.BotScopes("slack", nil),
		Account: slackTeam, AccountName: "Acme", App: slackApp,
		Raw: map[string]any{"bot_user_id": slackBotUser, "is_enterprise_install": false},
	})
	require.NoError(s.t, err)

	parsed, err := url.Parse(callback)
	require.NoError(s.t, err)

	landing := s.Owner.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusFound)

	location, err := url.Parse(landing.Header.Get("Location"))
	require.NoError(s.t, err)
	require.NotEmpty(s.t, location.Query().Get("connected"), location.String())
	s.TC.Drain()

	return location.Query().Get("connected")
}

/*
channelDestination makes a Slack channel destination hearing about the
triggers and returns its public id.
*/
func (s *slackDesk) channelDestination(integration string, body testkit.JSON) string {
	s.t.Helper()

	body["type"], body["integration_id"], body["channel_id"] = "slack_channel", integration, supportRoom

	return s.Owner.PostJSON("/api/v1/notification-destinations", body).AssertCreated().String("data.id")
}

/*
posts returns the cards posted in the Slack channel, as their fallback
text and blocks.
*/
func (s *slackDesk) posts(channel string) []slackCall {
	s.t.Helper()

	var out []slackCall

	for _, c := range s.slack.called("chat.postMessage") {
		if c.Form.Get("channel") == channel {
			out = append(out, c)
		}
	}

	return out
}

/*
blocks reads the blocks of a posted card.
*/
func blocks(t *testing.T, c slackCall) []map[string]any {
	t.Helper()

	var out []map[string]any
	require.NoError(t, json.Unmarshal([]byte(c.Form.Get("blocks")), &out))

	return out
}

/*
destination reads a destination's row.
*/
func (s *slackDesk) destination(public string) models.NotificationDestination {
	s.t.Helper()

	id, err := s.TC.App.IDs.Decode(string(models.PrefixDestination), public)
	require.NoError(s.t, err)

	var row models.NotificationDestination

	ctx := tenancy.With(s.t.Context(), s.TC.Workspace.Workspace.ID.String())
	require.NoError(s.t, s.TC.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err = models.New(tx).FindNotificationDestination(ctx, models.FindNotificationDestinationParams{
			WorkspaceID: s.TC.Workspace.Workspace.ID, ID: id,
		})

		return err
	}))

	return row
}
