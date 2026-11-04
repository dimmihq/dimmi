package emaildomains_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
inboundSecret opens the test inbound source.
*/
const inboundSecret = "domains-inbound-secret"

/*
inboundEnv receives mail through the test source.
*/
func inboundEnv() tests.Option {
	return tests.WithEnv(
		"DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=test",
		"DIMMI_CHANNELS__EMAIL__INBOUND__TEST_SECRET="+inboundSecret,
	)
}

/*
conversationFrom delivers a first email from the customer to the
address and returns its conversation.
*/
func conversationFrom(tc *tests.TestCase, forwarding, customer string) models.Conversation {
	tc.T.Helper()

	raw := "From: Bob <" + customer + ">\r\nTo: support@acme.test\r\nSubject: Refund\r\n" +
		"Message-ID: <first." + ids.New().String() + "@customer.test>\r\n\r\nCharged twice.\r\n"
	tc.Client().WithHeader("Authorization", "Bearer "+inboundSecret).
		Send("POST", "/webhooks/test/inbound?to="+url.QueryEscape(forwarding), []byte(raw)).AssertStatus(202)
	tc.Drain()

	ws := tc.Workspace.Workspace.ID
	convs := query(tc, ws, func(ctx context.Context, q *models.Queries) ([]models.Conversation, error) {
		return q.ListConversations(ctx, models.ListConversationsParams{WorkspaceID: ws, RowLimit: 10})
	})
	require.NotEmpty(tc.T, convs)

	return convs[0]
}

/*
replyTo sends a member's reply on the conversation through the email
channel.
*/
func replyTo(tc *tests.TestCase, c models.Conversation, customer string) {
	tc.T.Helper()

	ws, body := c.WorkspaceID, "The refund is on its way."
	type sent struct {
		msg  models.Message
		conn models.ChannelConnection
	}

	got := query(tc, ws, func(ctx context.Context, q *models.Queries) (sent, error) {
		msg, err := q.CreateMessage(ctx, models.CreateMessageParams{
			WorkspaceID: ws, ConversationID: c.ID, Type: string(enums.MessageTypeMessage),
			AuthorType: string(enums.AuthorTypeMember), AuthorID: &tc.Workspace.Owner.ID, Body: &body,
			DeliveryStatus: new(string(enums.DeliveryStatusPending)), Substantive: true, Metadata: []byte(`{}`),
			Now: tc.Clock.Now(),
		})
		if err != nil {
			return sent{}, err
		}

		conn, err := q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: ws, ID: c.ChannelConnectionID,
		})

		return sent{msg: msg, conn: conn}, err
	})
	msg, conn := got.msg, got.conn

	_, err := tc.App.EmailChannel.Send(tc.T.Context(), conn, contracts.OutboundMessage{
		WorkspaceID: ws, ConversationID: c.ID, ConversationNumber: c.Number, MessageID: msg.ID,
		IdempotencyKey: models.PublicID(models.PrefixMessage, msg.ID), Subject: *c.Subject,
		BodyMarkdown: body, BodyText: body, BodyHTML: "<p>" + body + "</p>",
		Recipient: contracts.Recipient{
			ContactID: c.ContactID, Name: "Bob", IdentityType: enums.IdentityTypeEmail, IdentityValue: customer,
		},
		Author: contracts.OutboundAuthor{Type: enums.AuthorTypeMember, ID: tc.Workspace.Owner.ID, Name: "Sam"},
	})
	require.NoError(tc.T, err)
}

/*
mailbox returns the raw headers Mailpit holds for the recipient.
*/
func mailbox(t *testing.T, recipient string) string {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)
	api := testkit.Env(t, "MAILPIT_API_URL")

	resp, err := client.New().Get(t.Context(), api+"/api/v1/search?query="+url.QueryEscape(`to:"`+recipient+`"`))
	require.NoError(t, err)

	var found struct{ Messages []struct{ ID string } }
	require.NoError(t, resp.JSON(&found))
	require.Len(t, found.Messages, 1)

	resp, err = client.New().Get(t.Context(), api+"/api/v1/message/"+found.Messages[0].ID+"/headers")
	require.NoError(t, err)

	return string(resp.Body)
}

func TestRepliesFromOwnDomain(t *testing.T) {
	t.Parallel()

	t.Run("replies from the customer's own address once its domain is verified", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t), inboundEnv())
		owner := tc.ActingAsOwner()
		address := owner.PostJSON("/api/v1/email-addresses", map[string]any{
			"address": "support@acme.test", "display_name": "Acme Support",
		}).AssertCreated()
		address.AssertJSONPath("data.sending_state", "default_sender")
		forwarding := address.String("data.forwarding_address")
		_, created := verifiedRelayDomain(tc, zone)

		shown := owner.GetJSON("/api/v1/email-addresses/" + address.String("data.id")).AssertOK()
		shown.AssertJSONPath("data.sending_state", "own_domain")
		shown.AssertJSONPath("data.sends_as", `"Acme Support" <support@acme.test>`)
		shown.AssertJSONPath("data.sending_identity.uses_default_sender", false)
		shown.AssertJSONPath("data.sending_blocked_reason", nil)

		customer := "bob." + strings.ToLower(ids.New().String()[:8]) + "@customer.test"
		replyTo(tc, conversationFrom(tc, forwarding, customer), customer)

		headers := mailbox(t, customer)
		assert.Contains(t, headers, "support@acme.test")
		assert.Contains(t, headers, "d=acme.test")
		assert.Contains(t, headers, "s="+selectorOf(created))
	})

	t.Run("falls back to the forwarding address once the domain degrades", func(t *testing.T) {
		t.Parallel()
		tc, zone := domainCase(t, relayEnv(t), inboundEnv())
		owner := tc.ActingAsOwner()
		address := owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "help@acme.test"}).
			AssertCreated()
		forwarding := address.String("data.forwarding_address")
		_, created := verifiedRelayDomain(tc, zone)
		zone.Set("_dimmi-verification.acme.test", dns.Records{})
		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)
		tc.Clock.Travel(time.Hour)
		check(tc, tc.Workspace.Workspace.ID, created.Data.ID)

		owner.GetJSON("/api/v1/email-addresses/"+address.String("data.id")).
			AssertJSONPath("data.sending_state", "default_sender").
			AssertJSONPath("data.sending_blocked_reason", "domain_degraded")

		customer := "ann." + strings.ToLower(ids.New().String()[:8]) + "@customer.test"
		replyTo(tc, conversationFrom(tc, forwarding, customer), customer)

		headers := mailbox(t, customer)
		assert.Contains(t, headers, forwarding)
		assert.NotContains(t, headers, "d=acme.test")
	})
}

/*
selectorOf returns the selector of the domain's signing key record.
*/
func selectorOf(body domainBody) string {
	for _, r := range body.Data.Records {
		if r.Purpose == "dkim" {
			return strings.TrimSuffix(r.Name, "._domainkey.acme.test")
		}
	}

	return ""
}
