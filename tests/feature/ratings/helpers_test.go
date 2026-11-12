package ratings_test

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
inWorkspace runs a read or write inside the desk's workspace.
*/
func inWorkspace(t *testing.T, d tests.Desk, fn func(ctx context.Context, q *models.Queries) error) {
	t.Helper()

	ws := d.TC.Workspace.Workspace.ID
	err := d.TC.App.DB.Transaction(tenancy.With(t.Context(), ws.String()),
		func(ctx context.Context, tx *database.Tx) error { return fn(ctx, models.New(tx)) })
	require.NoError(t, err)
}

/*
emailDesk is a desk whose workspace receives mail on a test inbound
domain, so it has a default address for replies to leave from.
*/
func emailDesk(t *testing.T) tests.Desk {
	t.Helper()

	tc := tests.New(t, tests.WithEnv("DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=inbound.test"))
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	require.NoError(t, err)

	return tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
}

/*
settings changes the rating settings, recording mail from then on.
*/
func settings(d tests.Desk, change testkit.JSON) {
	d.TC.Mail()
	d.Owner.PatchJSON("/api/v1/conversation-rating-settings", change).AssertOK()
}

/*
visitor is a browser with a messenger session on the workspace's first
widget, which can start a conversation.
*/
func visitor(t *testing.T, d tests.Desk) *testkit.Client {
	t.Helper()

	var app string
	inWorkspace(t, d, func(ctx context.Context, q *models.Queries) error {
		rows, err := q.ListWidgetSettings(ctx, d.TC.Workspace.Workspace.ID)
		if err == nil {
			app = models.PublicID(models.PrefixWidget, rows[0].ID)
		}
		return err
	})

	res := d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": app}).AssertCreated()

	return d.TC.Client().WithHeader("Authorization", "Bearer "+res.String("data.token"))
}

/*
startChat has the visitor write a first message and returns the
conversation's public id.
*/
func startChat(v *testkit.Client) string {
	return v.PostJSON("/widget/v1/conversations", testkit.JSON{
		"client_id": ids.New().String(), "body": "Hello",
	}).AssertCreated().String("data.id")
}

/*
closeChat has the owner reply to the conversation, so it is theirs, and
close it, then runs the jobs the close queued.
*/
func closeChat(d tests.Desk, id string) {
	d.Owner.PostJSON("/api/v1/conversations/"+id+"/replies", testkit.JSON{"body": "Fixed it"}).AssertCreated()
	d.Owner.PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()
	d.TC.Drain()
}

/*
emailChat opens a conversation through the API and moves it onto an
email channel of the same inbox with the customer's first email filed
under it, the way one that came in by email lives, returning its
public id.
*/
func emailChat(t *testing.T, d tests.Desk, email string) string {
	t.Helper()

	id := d.Owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"inbox_id": d.InboxID(), "author": "contact", "subject": "Help", "body": "Hi",
		"contact": testkit.JSON{"email": email, "name": "Ann"},
	}).AssertCreated().String("data.id")
	d.TC.Drain()

	conv, err := d.TC.App.IDs.Decode(string(models.PrefixConversation), id)
	require.NoError(t, err)

	_, err = d.TC.Operator().Exec(t.Context(), `
		WITH email AS (
			INSERT INTO channel_connections (workspace_id, inbox_id, type, name, status)
			SELECT workspace_id, inbox_id, 'email', 'Support email', 'active' FROM conversations WHERE id = $1
			RETURNING id
		)
		UPDATE conversations SET channel_connection_id = (SELECT id FROM email) WHERE id = $1`, conv)
	require.NoError(t, err)

	_, err = d.TC.Operator().Exec(t.Context(), `
		INSERT INTO email_message_refs (workspace_id, rfc_message_id, conversation_id, kind)
		SELECT workspace_id, 'first-'||$2||'@mail.example.com', id, 'inbound' FROM conversations WHERE id = $1`,
		conv, email)
	require.NoError(t, err)

	return id
}

/*
ratingMail drains the queue and returns the rating emails the customer
stream sent, oldest first.
*/
func ratingMail(d tests.Desk) []mail.SentMessage {
	d.TC.Drain()

	var out []mail.SentMessage
	for _, sent := range d.TC.Mail().Sent() {
		if sent.Message.Tags["kind"] == "conversation_rating" {
			out = append(out, sent)
		}
	}

	return out
}

/*
defaultAddress returns the workspace's default email address, the one
an email conversation's replies leave from.
*/
func defaultAddress(t *testing.T, d tests.Desk) models.EmailAddress {
	t.Helper()

	var out models.EmailAddress
	inWorkspace(t, d, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.FindDefaultEmailAddress(ctx, d.TC.Workspace.Workspace.ID)
		return err
	})

	return out
}

/*
ratingToken returns the token of the link behind the face with the
score in the last rating email, checking it went through the customer
stream from the address the replies use.
*/
func ratingToken(t *testing.T, d tests.Desk, score int) string {
	t.Helper()

	sent := ratingMail(d)
	require.NotEmpty(t, sent, "no rating email was sent")
	last := sent[len(sent)-1]
	require.Equal(t, "customer", last.Mailer)
	require.Equal(t, defaultAddress(t, d).ForwardingAddress, last.Message.From.Email)
	require.Contains(t, last.Message.Headers["Message-ID"], "crt_", "the email is filed under the ask")
	require.Contains(t, last.Message.Headers["In-Reply-To"], "first-", "the email answers the customer's thread")

	var kind string
	require.NoError(t, d.TC.Operator().QueryRow(t.Context(),
		`SELECT kind FROM outbound_emails WHERE workspace_id = $1 AND from_address = $2
		 ORDER BY created_at DESC LIMIT 1`,
		d.TC.Workspace.Workspace.ID, last.Message.From.Email).Scan(&kind))
	require.Equal(t, "conversation_rating", kind)

	match := regexp.MustCompile(`href="([^"]*/rate\?t=[^"]*score=` + strconv.Itoa(score) + `)"`).
		FindStringSubmatch(last.Message.HTML)
	require.Len(t, match, 2)

	link, err := url.Parse(html.UnescapeString(match[1]))
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(link.Path, "/rate"))

	return link.Query().Get("t")
}
