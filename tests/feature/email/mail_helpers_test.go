package email_test

import (
	"bytes"
	"context"
	"io/fs"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail/dkim"
	"github.com/dimmihq/dimmi/framework/mailbox/parse"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/email"
)

/*
email is a raw email a test writes: its sender and recipients, subject,
ids and body, and any extra header lines.
*/
type email struct {
	from, to, cc, subject string
	id, inReplyTo         string
	references            []string
	headers               []string
	body                  string
}

/*
raw writes the email as a plain text message.
*/
func (e email) raw() []byte {
	var b bytes.Buffer

	b.WriteString("From: " + e.from + "\r\nTo: " + e.to + "\r\n")
	if e.cc != "" {
		b.WriteString("Cc: " + e.cc + "\r\n")
	}

	b.WriteString("Subject: " + e.subject + "\r\nDate: Tue, 3 Nov 2026 10:00:00 +0000\r\n")
	if e.id != "" {
		b.WriteString("Message-ID: <" + e.id + ">\r\n")
	}

	if e.inReplyTo != "" {
		b.WriteString("In-Reply-To: <" + e.inReplyTo + ">\r\n")
	}

	if len(e.references) > 0 {
		b.WriteString("References: <" + strings.Join(e.references, "> <") + ">\r\n")
	}

	for _, header := range e.headers {
		b.WriteString(header + "\r\n")
	}

	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + e.body + "\r\n")

	return b.Bytes()
}

/*
unique returns an address no other test uses, so Mailpit, shared by
every test, holds only this test's mail for it.
*/
func unique(name string) string {
	return name + "." + testkit.UniqueName("") + "@customer.test"
}

/*
mailpitEnv sends the customer stream through Mailpit.
*/
func mailpitEnv(t *testing.T) tests.Option {
	t.Helper()

	server, err := url.Parse(testkit.Env(t, "MAIL_SMTP_URL"))
	require.NoError(t, err)

	return tests.WithEnv(
		"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=smtp",
		"DIMMI_MAIL__MAILERS__CUSTOMER__HOST="+server.Hostname(),
		"DIMMI_MAIL__MAILERS__CUSTOMER__PORT="+server.Port(),
		"DIMMI_MAIL__MAILERS__CUSTOMER__ALLOW_INSECURE=true",
	)
}

/*
received returns the raw messages Mailpit holds for the recipient,
newest first.
*/
func received(t *testing.T, recipient string) [][]byte {
	t.Helper()

	return mailpitSearch(t, `to:"`+recipient+`"`)
}

/*
copied returns the raw messages Mailpit holds that copied the address,
newest first.
*/
func copied(t *testing.T, recipient string) [][]byte {
	t.Helper()

	return mailpitSearch(t, `cc:"`+recipient+`"`)
}

/*
mailpitSearch returns the raw messages matching the Mailpit search,
newest first.
*/
func mailpitSearch(t *testing.T, query string) [][]byte {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)

	api := testkit.Env(t, "MAILPIT_API_URL")
	search := url.Values{"query": {query}}

	resp, err := client.New().Get(t.Context(), api+"/api/v1/search?"+search.Encode())
	require.NoError(t, err)

	var found struct{ Messages []struct{ ID string } }
	require.NoError(t, resp.JSON(&found))

	out := make([][]byte, 0, len(found.Messages))
	for _, m := range found.Messages {
		raw, err := client.New().Get(t.Context(), api+"/api/v1/message/"+m.ID+"/raw")
		require.NoError(t, err)

		out = append(out, raw.Body)
	}

	return out
}

/*
parsed parses a raw message.
*/
func parsed(t *testing.T, raw []byte) *parse.Parsed {
	t.Helper()

	msg, err := parse.Message(bytes.NewReader(raw), parse.Limits{})
	require.NoError(t, err)

	return msg
}

/*
signed signs the raw email with the test DKIM key for the domain, and
publishes the key in the fake zone.
*/
func signed(t *testing.T, zone *dns.Fake, domain string, raw []byte) []byte {
	t.Helper()

	key, err := fs.ReadFile(fixtures.Files, "dkim_private.pem")
	require.NoError(t, err)
	public, err := fs.ReadFile(fixtures.Files, "dkim_public.txt")
	require.NoError(t, err)

	zone.Set("sel._domainkey."+domain, dns.Records{TXT: []string{"v=DKIM1; k=rsa; p=" + string(public)}})

	signer, err := dkim.New(dkim.Options{Domain: domain, Selector: "sel", PrivateKey: string(key)})
	require.NoError(t, err)

	out, err := signer.Sign(raw)
	require.NoError(t, err)

	return out
}

/*
reply is a teammate's reply stored in a conversation, ready for the
channel to send.
*/
type reply struct {
	conversation models.Conversation
	message      models.Message
	connection   models.ChannelConnection
	outbound     contracts.OutboundMessage
}

/*
memberReply stores a teammate's reply to the conversation's contact and
returns it with the outbound message the inbox hands the channel.
*/
func memberReply(tc *tests.TestCase, conversation models.Conversation, recipient string) reply {
	tc.T.Helper()

	ws := conversation.WorkspaceID
	body := "Hi, the refund is on its way."

	r := query(tc, ws, func(ctx context.Context, q *models.Queries) (reply, error) {
		msg, err := q.CreateMessage(ctx, models.CreateMessageParams{
			WorkspaceID: ws, ConversationID: conversation.ID, Type: string(enums.MessageTypeMessage),
			AuthorType: string(enums.AuthorTypeMember), AuthorID: &tc.Workspace.Owner.ID, Body: &body,
			DeliveryStatus: new(string(enums.DeliveryStatusPending)), Substantive: true, Metadata: []byte(`{}`),
			Now: tc.Clock.Now(),
		})
		if err != nil {
			return reply{}, err
		}

		conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: ws, ID: conversation.ChannelConnectionID,
		})

		return reply{conversation: conversation, message: msg, connection: conn}, err
	})

	key := ""
	if conversation.ExternalThreadKey != nil {
		key = *conversation.ExternalThreadKey
	}

	r.outbound = contracts.OutboundMessage{
		WorkspaceID: ws, ConversationID: conversation.ID, ConversationNumber: conversation.Number,
		MessageID: r.message.ID, IdempotencyKey: models.PublicID(models.PrefixMessage, r.message.ID),
		ExternalThreadKey: key, Subject: *conversation.Subject, BodyMarkdown: body, BodyText: body,
		BodyHTML: "<p>" + body + "</p>",
		Recipient: contracts.Recipient{
			ContactID: conversation.ContactID, Name: "Bob Stone", IdentityType: enums.IdentityTypeEmail,
			IdentityValue: recipient,
		},
		Author: contracts.OutboundAuthor{Type: enums.AuthorTypeMember, ID: tc.Workspace.Owner.ID, Name: "Sam"},
	}

	return r
}

/*
send hands the reply to the email channel.
*/
func send(tc *tests.TestCase, r reply) (contracts.SendResult, error) {
	tc.T.Helper()

	return tc.App.EmailChannel.Send(tc.T.Context(), r.connection, r.outbound)
}

/*
startConversation delivers a first email from the customer and returns
its conversation.
*/
func startConversation(tc *tests.TestCase, address models.EmailAddress, customer string) models.Conversation {
	tc.T.Helper()

	deliver(tc, email{
		from: "Bob Stone <" + customer + ">", to: "support@acme.test", subject: "Refund for order 1042",
		id: "first." + ids.New().String() + "@customer.test", body: "I was charged twice.",
		headers: []string{"Delivered-To: " + address.ForwardingAddress},
	}.raw(), to(address.ForwardingAddress))

	convs := conversations(tc)
	require.NotEmpty(tc.T, convs)

	return convs[0]
}
