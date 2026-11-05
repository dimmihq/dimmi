package emaildomains_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
teammate adds a member with the member role and signs them in.
*/
func teammate(tc *tests.TestCase) (models.WorkspaceMember, *testkit.Client) {
	tc.T.Helper()

	user := tc.CreateUser()
	member, err := factories.Member(tc.T.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
	})
	require.NoError(tc.T, err)

	return member, tc.ActingAs(user)
}

/*
signedReply sends a reply on the conversation by the author, with the
email options given, and returns the mail that left.
*/
func signedReply(
	tc *tests.TestCase, c models.Conversation, author contracts.OutboundAuthor, options string,
) mail.Message {
	tc.T.Helper()

	fake := tc.Mail()
	before := len(fake.Sent())
	ws, body := c.WorkspaceID, "The refund is on its way."

	msg := query(tc, ws, func(ctx context.Context, q *models.Queries) (models.Message, error) {
		return q.CreateMessage(ctx, models.CreateMessageParams{
			WorkspaceID: ws, ConversationID: c.ID, Type: string(enums.MessageTypeMessage),
			AuthorType: string(author.Type), AuthorID: &author.ID, Body: &body,
			DeliveryStatus: new(string(enums.DeliveryStatusPending)), Substantive: true, Metadata: []byte(`{}`),
			Now: tc.Clock.Now(),
		})
	})
	conn := query(tc, ws, func(ctx context.Context, q *models.Queries) (models.ChannelConnection, error) {
		return q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
			WorkspaceID: ws, ID: c.ChannelConnectionID,
		})
	})

	_, err := tc.App.EmailChannel.Send(tc.T.Context(), conn, contracts.OutboundMessage{
		WorkspaceID: ws, ConversationID: c.ID, ConversationNumber: c.Number, MessageID: msg.ID,
		IdempotencyKey: models.PublicID(models.PrefixMessage, msg.ID), Subject: *c.Subject,
		BodyMarkdown: body, BodyText: body, BodyHTML: "<p>" + body + "</p>", Options: []byte(options),
		Recipient: contracts.Recipient{
			ContactID: c.ContactID, Name: "Bob", IdentityType: enums.IdentityTypeEmail,
			IdentityValue: "bob@customer.test",
		},
		Author: author,
	})
	require.NoError(tc.T, err)

	sent := fake.Sent()
	require.Len(tc.T, sent, before+1)

	return sent[before].Message
}

/*
signatureCase is a workspace with one address, a conversation on it
and a teammate, ready for replies.
*/
type signatureCase struct {
	tc       *tests.TestCase
	owner    *testkit.Client
	address  string
	conv     models.Conversation
	member   models.WorkspaceMember
	asMember *testkit.Client
}

/*
newSignatureCase sets up the address, the conversation and a teammate.
*/
func newSignatureCase(t *testing.T) signatureCase {
	t.Helper()

	tc, _ := domainCase(t, inboundEnv())
	owner := tc.ActingAsOwner()
	address := owner.PostJSON("/api/v1/email-addresses", map[string]any{
		"address": "support@acme.test", "display_name": "Acme Support",
	}).AssertCreated()
	customer := "bob." + strings.ToLower(ids.New().String()[:8]) + "@customer.test"
	conv := conversationFrom(tc, address.String("data.forwarding_address"), customer)
	tc.Mail()
	member, asMember := teammate(tc)

	return signatureCase{
		tc: tc, owner: owner, address: address.String("data.id"), conv: conv, member: member, asMember: asMember,
	}
}

/*
memberAuthor is the teammate writing a reply.
*/
func (s signatureCase) memberAuthor(m models.WorkspaceMember, name string) contracts.OutboundAuthor {
	return contracts.OutboundAuthor{Type: enums.AuthorTypeMember, ID: m.ID, Name: name}
}

func TestEmailSignatures(t *testing.T) {
	t.Parallel()

	t.Run("lets a teammate keep their own signature but not the team's", func(t *testing.T) {
		t.Parallel()
		s := newSignatureCase(t)

		own := s.asMember.PostJSON("/api/v1/email-signatures", map[string]any{"body": "Thanks,\n**{{member_name}}**"}).
			AssertCreated()
		own.AssertJSONPath("data.object", "email_signature").
			AssertJSONPath("data.kind", "personal").
			AssertJSONPath("data.member_id", models.PublicID(models.PrefixMember, s.member.ID)).
			AssertJSONPath("data.email_address_id", nil).
			AssertJSONPath("data.enabled", true)

		s.asMember.PostJSON("/api/v1/email-signatures", map[string]any{
			"kind": "address", "email_address_id": s.address, "body": "The Acme team",
		}).AssertStatus(403)
		s.asMember.PostJSON("/api/v1/email-signatures", map[string]any{"body": "Again"}).
			AssertStatus(409).AssertJSONPath("code", "signature_exists")

		team := s.owner.PostJSON("/api/v1/email-signatures", map[string]any{
			"kind": "address", "email_address_id": s.address, "body": "The {{address_name}} team",
		}).AssertCreated()
		team.AssertJSONPath("data.kind", "address").AssertJSONPath("data.member_id", nil)
		s.owner.PostJSON("/api/v1/email-signatures", map[string]any{"body": "Owner"}).AssertCreated()

		s.asMember.GetJSON("/api/v1/email-signatures").AssertOK().AssertJSONPath("data.#", 2)
		s.owner.GetJSON("/api/v1/email-signatures").AssertOK().AssertJSONPath("data.#", 3)
		s.owner.GetJSON("/api/v1/email-signatures?filter[kind]=address").AssertJSONPath("data.#", 1)
		s.owner.GetJSON("/api/v1/email-signatures?filter[member]=me").AssertJSONPath("data.#", 1)
		s.owner.GetJSON("/api/v1/email-signatures?filter[address]="+s.address).AssertJSONPath("data.#", 1)

		s.asMember.PatchJSON("/api/v1/email-signatures/"+team.String("data.id"), map[string]any{"enabled": false}).
			AssertStatus(403)
		s.asMember.PatchJSON("/api/v1/email-signatures/"+own.String("data.id"), map[string]any{"enabled": false}).
			AssertOK().AssertJSONPath("data.enabled", false)
		s.asMember.Delete("/api/v1/email-signatures/" + own.String("data.id")).AssertStatus(204)
		s.asMember.GetJSON("/api/v1/email-signatures?filter[member]=me").AssertJSONPath("data.#", 0)
	})

	t.Run("refuses placeholders no reply can fill and team signatures without an address", func(t *testing.T) {
		t.Parallel()
		s := newSignatureCase(t)

		s.owner.PostJSON("/api/v1/email-signatures", map[string]any{"body": "{{member_name}} {{nickname}}"}).
			AssertStatus(422).
			AssertJSONPath("code", "unknown_placeholder").
			AssertJSONPath("placeholders", []any{"{{nickname}}"}).
			AssertJSONPath("errors.body.#", 1)
		s.owner.PostJSON("/api/v1/email-signatures", map[string]any{"kind": "address", "body": "Team"}).
			AssertStatus(422).AssertJSONPath("code", "address_required")
		s.owner.PostJSON("/api/v1/email-signatures", map[string]any{
			"kind": "address", "email_address_id": "ema_missing", "body": "Team",
		}).AssertStatus(404)
		s.owner.GetJSON("/api/v1/email-signatures?filter[kind]=other").AssertStatus(400)
	})

	t.Run("previews a signature as the caller's reply shows it", func(t *testing.T) {
		t.Parallel()
		s := newSignatureCase(t)

		shown := s.owner.PostJSON("/api/v1/email-signatures/renderings", map[string]any{
			"body":             "**{{member_name}}**\n{{member_title}}\n{{address_name}} at [Acme](https://acme.test)\n<script>x</script>",
			"email_address_id": s.address,
		}).AssertCreated()
		shown.AssertJSONPath("data.object", "email_signature_rendering")

		html, text := shown.String("data.html"), shown.String("data.text")
		assert.Contains(t, html, "<strong>"+s.tc.Owner.Name+"</strong>")
		assert.Contains(t, html, `<a href="https://acme.test"`)
		assert.NotContains(t, html, "<script")
		assert.Equal(t, "-- \n"+s.tc.Owner.Name+"\nAcme Support at [Acme](https://acme.test)\n<script>x</script>",
			strings.ReplaceAll(text, "**", ""))
	})

	t.Run("signs a teammate's reply with the most specific signature they have", func(t *testing.T) {
		t.Parallel()
		s := newSignatureCase(t)
		addressID := s.address

		s.owner.PostJSON("/api/v1/email-signatures", map[string]any{
			"kind": "address", "email_address_id": addressID, "body": "The {{address_name}} team",
		}).AssertCreated()
		author := s.memberAuthor(s.member, "Sam")

		reply := signedReply(s.tc, s.conv, author, "")
		assert.Contains(t, reply.Text, "-- \nThe Acme Support team")

		general := s.asMember.PostJSON("/api/v1/email-signatures", map[string]any{"body": "Sam, everywhere"}).
			AssertCreated()
		assert.Contains(t, signedReply(s.tc, s.conv, author, "").Text, "-- \nSam, everywhere")

		s.asMember.PostJSON("/api/v1/email-signatures", map[string]any{
			"email_address_id": addressID, "body": "Sam at support",
		}).AssertCreated()
		reply = signedReply(s.tc, s.conv, author, "")
		assert.Contains(t, reply.Text, "The refund is on its way.\n\n-- \nSam at support")
		assert.Contains(t, reply.HTML, "Sam at support")

		assert.NotContains(t, signedReply(s.tc, s.conv, author, `{"email":{"signature":false}}`).Text, "-- ")

		s.asMember.PatchJSON("/api/v1/email-signatures/"+general.String("data.id"), map[string]any{"enabled": false})
		assert.Contains(t, signedReply(s.tc, s.conv, author, "").Text, "Sam at support")
	})

	t.Run("signs replies nobody wrote by hand and those of a teammate who left with the team's", func(t *testing.T) {
		t.Parallel()
		s := newSignatureCase(t)

		s.asMember.PostJSON("/api/v1/email-signatures", map[string]any{"body": "Sam"}).AssertCreated()
		agent := contracts.OutboundAuthor{Type: enums.AuthorTypeAgent, ID: ids.New(), Name: "Assistant"}
		assert.NotContains(t, signedReply(s.tc, s.conv, agent, "").Text, "-- ")

		s.owner.PostJSON("/api/v1/email-signatures", map[string]any{
			"kind": "address", "email_address_id": s.address, "body": "{{company_name}} support",
		}).AssertCreated()
		reply := signedReply(s.tc, s.conv, agent, "")
		assert.Contains(t, reply.Text, "-- \n"+s.tc.Workspace.Workspace.Name+" support")

		require.NoError(t, factories.Deactivate(t.Context(), s.tc.App, s.member))
		reply = signedReply(s.tc, s.conv, s.memberAuthor(s.member, "Sam"), "")
		assert.Contains(t, reply.Text, "-- \n"+s.tc.Workspace.Workspace.Name+" support")
	})
}
