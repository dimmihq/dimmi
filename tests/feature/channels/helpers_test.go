package channels_test

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/channels/slack"
)

/*
The Slack app the tests connect: its signing secret and tokens, the
installing team, the app and the bot.
*/
const (
	slackSigning  = "test-signing-secret"
	slackBot      = "xoxb-fixture"
	installTeam   = "T0ACME"
	customerTeam  = "T0BETA"
	appID         = "A0APP"
	botUser       = "U0BOT"
	supportRoom   = "C0SUPPORT"
	communityRoom = "C0COMMUNITY"
	rootTS        = "1793000000.000100"
	secondTS      = "1793000010.000100"
)

/*
slackCall is one Web API call the fake Slack received, with the token
it was made with and its raw body for calls sent as JSON.
*/
type slackCall struct {
	Method string
	Form   url.Values
	Token  string
	Body   string
}

/*
slackAnswer is an answer a test scripts for a method instead of its
recorded one.
*/
type slackAnswer struct {
	status int
	body   string
	header map[string]string
}

/*
fakeSlack answers the Web API from the recorded fixtures and records
every call.
*/
type fakeSlack struct {
	t          testing.TB
	ownerEmail string

	mu      sync.Mutex
	calls   []slackCall
	answers map[string][]slackAnswer
}

/*
slackCase is a test with Slack faked and a workspace's own app
connected, its rooms read.
*/
type slackCase struct {
	*tests.TestCase

	slack  *fakeSlack
	conn   models.ChannelConnection
	events int
}

/*
newSlackCase boots the application with Slack faked, without
connecting anything.
*/
func newSlackCase(t *testing.T, opts ...tests.Option) *slackCase {
	t.Helper()

	tc := tests.New(t, opts...)
	fake := &fakeSlack{t: t, ownerEmail: tc.Owner.Email, answers: map[string][]slackAnswer{}}

	httpclienttest.New(tc.App.HTTP).
		Serve("https://slack.com/api/*", fhttp.HandlerFunc(fake.serve)).
		Serve("https://files.slack.com/*", fhttp.HandlerFunc(fake.serveFile))

	return &slackCase{TestCase: tc, slack: fake}
}

/*
connectedCase boots the application with a workspace's own Slack app
connected and its rooms read.
*/
func connectedCase(t *testing.T, opts ...tests.Option) *slackCase {
	t.Helper()

	sc := newSlackCase(t, opts...)
	sc.conn = connectNow(sc)

	return sc
}

/*
connectNow connects the app with its secrets and runs the room sync it
queues.
*/
func connectNow(sc *slackCase) models.ChannelConnection {
	sc.T.Helper()

	res := sc.ActingAsOwner().PostJSON("/api/v1/channels/slack", map[string]any{
		"name": "Acme Slack", "bot_token": slackBot, "signing_secret": slackSigning,
	}).AssertCreated()

	id, err := sc.App.IDs.Decode(string(models.PrefixChannelConnection), res.String("data.id"))
	require.NoError(sc.T, err)

	sc.Drain()

	return sc.connection(id)
}

/*
serve answers one Web API call.
*/
func (f *fakeSlack) serve(w fhttp.ResponseWriter, r *fhttp.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/api/")
	var raw []byte
	if r.Body != nil {
		raw, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(raw))
	}
	_ = r.ParseForm()

	f.mu.Lock()
	token := cmp.Or(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), r.Form.Get("token"))
	f.calls = append(f.calls, slackCall{Method: method, Form: r.Form, Token: token, Body: string(raw)})

	scripted, ok := f.answers[method]
	if ok && len(scripted) > 0 {
		f.answers[method] = scripted[1:]
	}
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if ok && len(scripted) > 0 {
		for name, value := range scripted[0].header {
			w.Header().Set(name, value)
		}

		w.WriteHeader(scripted[0].status)
		_, _ = io.WriteString(w, scripted[0].body)

		return
	}

	name := method + ".json"
	if method == "users.info" {
		name = "users.info." + r.Form.Get("user") + ".json"
	}

	raw, err := fs.ReadFile(fixtures.Files, name)
	if err != nil {
		_, _ = io.WriteString(w, `{"ok": false, "error": "user_not_found"}`)

		return
	}

	_, _ = io.WriteString(w, strings.ReplaceAll(string(raw), "{{owner_email}}", f.ownerEmail))
}

/*
serveFile accepts an uploaded file and serves a downloaded one.
*/
func (f *fakeSlack) serveFile(w fhttp.ResponseWriter, r *fhttp.Request) {
	f.mu.Lock()
	f.calls = append(f.calls, slackCall{Method: "file " + r.Method + " " + r.URL.Path})
	f.mu.Unlock()

	_, _ = io.WriteString(w, "file contents")
}

/*
answer scripts the next answers of the method.
*/
func (f *fakeSlack) answer(method string, answers ...slackAnswer) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.answers[method] = append(f.answers[method], answers...)
}

/*
called returns the calls of the method, oldest first.
*/
func (f *fakeSlack) called(method string) []slackCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []slackCall

	for _, call := range f.calls {
		if call.Method == method {
			out = append(out, call)
		}
	}

	return out
}

/*
connection returns the workspace's connection with the id.
*/
func (sc *slackCase) connection(id ids.UUID) models.ChannelConnection {
	sc.T.Helper()

	return query(sc, func(ctx context.Context, q *models.Queries) (models.ChannelConnection, error) {
		return q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: sc.Workspace.Workspace.ID, ID: id,
		})
	})
}

/*
query runs the function with the workspace's queries.
*/
func query[T any](sc *slackCase, fn func(ctx context.Context, q *models.Queries) (T, error)) T {
	sc.T.Helper()

	var out T

	ctx := tenancy.With(sc.T.Context(), sc.Workspace.Workspace.ID.String())
	err := sc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = fn(ctx, models.New(tx))

		return err
	})
	require.NoError(sc.T, err)

	return out
}

/*
publicID returns the connection's public id.
*/
func (sc *slackCase) publicID() string {
	return models.PublicID(models.PrefixChannelConnection, sc.conn.ID)
}

/*
signed posts the body to the path signed with the secret now, with the
content type.
*/
func (sc *slackCase) signed(path, secret, contentType string, body []byte) *testkit.Response {
	sc.T.Helper()

	ts := strconv.FormatInt(sc.Clock.Now().Unix(), 10)

	return sc.Client().WithHeader("Content-Type", contentType).
		WithHeader("X-Slack-Request-Timestamp", ts).
		WithHeader("X-Slack-Signature", slackdriver.SignatureHeader(secret, ts, body)).
		Send("POST", path, body)
}

/*
envelope wraps the event in an Events API delivery of the app in the
installing team.
*/
func envelope(eventID string, event map[string]any) []byte {
	raw, _ := json.Marshal(map[string]any{
		"type": "event_callback", "team_id": installTeam, "api_app_id": appID, "event_id": eventID,
		"event_time": 1793000000, "event": event,
		"authorizations": []map[string]any{{"team_id": installTeam, "user_id": botUser, "is_bot": true}},
	})

	return raw
}

/*
deliver posts the event to the connection's own address, signed, and
runs what it queued.
*/
func (sc *slackCase) deliver(event map[string]any) {
	sc.T.Helper()

	sc.events++
	sc.signed("/webhooks/slack/events/"+sc.publicID(), slackSigning, "application/json",
		envelope(fmt.Sprintf("Ev%04d", sc.events), event)).AssertOK()
	sc.Drain()
}

/*
interact posts the interaction to the connection's own address, signed,
and runs what it queued.
*/
func (sc *slackCase) interact(payload map[string]any) {
	sc.T.Helper()

	raw, err := json.Marshal(payload)
	require.NoError(sc.T, err)

	body := []byte(url.Values{"payload": {string(raw)}}.Encode())
	sc.signed("/webhooks/slack/interactions/"+sc.publicID(), slackSigning,
		"application/x-www-form-urlencoded", body).AssertOK()
	sc.Drain()
}

/*
message is a message event in the channel by the user at the timestamp.
*/
func message(channel, user, ts, text string) map[string]any {
	return map[string]any{
		"type": "message", "channel": channel, "channel_type": "channel", "user": user, "ts": ts, "text": text,
	}
}

/*
reply is a reply in the thread of the root.
*/
func reply(ev map[string]any, root string) map[string]any {
	ev["thread_ts"] = root

	return ev
}

/*
conversations returns the workspace's conversations on the connection,
oldest first.
*/
func (sc *slackCase) conversations() []models.Conversation {
	sc.T.Helper()

	rows, err := sc.Operator().Query(sc.T.Context(),
		"SELECT id FROM conversations WHERE workspace_id = $1 AND channel_connection_id = $2 "+
			"ORDER BY created_at, id",
		sc.Workspace.Workspace.ID, sc.conn.ID)
	require.NoError(sc.T, err)

	var found []ids.UUID

	for rows.Next() {
		var id ids.UUID
		require.NoError(sc.T, rows.Scan(&id))
		found = append(found, id)
	}

	rows.Close()

	out := make([]models.Conversation, 0, len(found))
	for _, id := range found {
		out = append(out, query(sc, func(ctx context.Context, q *models.Queries) (models.Conversation, error) {
			return q.FindConversation(ctx, models.FindConversationParams{
				WorkspaceID: sc.Workspace.Workspace.ID, ID: id,
			})
		}))
	}

	return out
}

/*
messages returns the conversation's messages, oldest first.
*/
func (sc *slackCase) messages(conv models.Conversation) []models.Message {
	sc.T.Helper()

	rows, err := sc.Operator().Query(sc.T.Context(),
		"SELECT id FROM messages WHERE workspace_id = $1 AND conversation_id = $2 ORDER BY created_at, id",
		sc.Workspace.Workspace.ID, conv.ID)
	require.NoError(sc.T, err)

	var found []ids.UUID

	for rows.Next() {
		var id ids.UUID
		require.NoError(sc.T, rows.Scan(&id))
		found = append(found, id)
	}

	rows.Close()

	out := make([]models.Message, 0, len(found))
	for _, id := range found {
		out = append(out, query(sc, func(ctx context.Context, q *models.Queries) (models.Message, error) {
			return q.FindWorkspaceMessage(ctx, models.FindWorkspaceMessageParams{
				WorkspaceID: sc.Workspace.Workspace.ID, ID: id,
			})
		}))
	}

	return out
}

/*
room returns the connection's room of the Slack channel.
*/
func (sc *slackCase) room(channel string) models.ChannelRoom {
	sc.T.Helper()

	return query(sc, func(ctx context.Context, q *models.Queries) (models.ChannelRoom, error) {
		return q.FindChannelRoomByExternalID(ctx, models.FindChannelRoomByExternalIDParams{
			WorkspaceID: sc.Workspace.Workspace.ID, ChannelConnectionID: sc.conn.ID, ExternalID: channel,
		})
	})
}

/*
setRoom changes how the room is watched.
*/
func (sc *slackCase) setRoom(channel, mode, trigger string) {
	sc.T.Helper()

	_, err := sc.Operator().Exec(sc.T.Context(),
		"UPDATE channel_rooms SET mode = $1, trigger = $2 WHERE channel_connection_id = $3 AND external_id = $4",
		mode, trigger, sc.conn.ID, channel)
	require.NoError(sc.T, err)
}

/*
body returns a message's text.
*/
func body(m models.Message) string {
	if m.Body == nil {
		return ""
	}

	return *m.Body
}

/*
refused is Slack's answer refusing a call with the error.
*/
func refused(code string) string {
	return `{"ok": false, "error": "` + code + `"}`
}
