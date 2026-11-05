package email_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestThreading(t *testing.T) {
	t.Parallel()

	t.Run("joins a reply by its subject when nothing else threads it", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, email{
			from: "Dana <dana@customer.test>", to: address.ForwardingAddress, subject: "Your delivery",
			id: "delivery-1@customer.test", body: "When does it ship?",
		}.raw(), "")

		deliver(tc, fixture(t, "apple_reply.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		msgs := messages(tc, convs[0].ID)
		require.Len(t, msgs, 2)
		assert.Equal(t, "subject", emailMetadata(t, msgs[1])["thread_method"])
	})

	t.Run("never threads a fresh email or a generic subject by subject", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		for i, subject := range []string{"Help", "Re: Help", "Billing question"} {
			deliver(tc, email{
				from: "Lee <lee@customer.test>", to: address.ForwardingAddress, subject: subject,
				id: "generic-" + string(rune('a'+i)) + "@customer.test", body: "Hello",
			}.raw(), "")
		}

		deliver(tc, email{
			from: "Lee <lee@customer.test>", to: address.ForwardingAddress, subject: "Billing question",
			id: "fresh-again@customer.test", body: "Hello again",
		}.raw(), "")

		assert.Len(t, conversations(tc), 4)
	})

	t.Run("finds the thread of a reference newer than the root", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "ref@customer.test")
		first := *messages(tc, conversation.ID)[0].ChannelMessageID

		deliver(tc, email{
			from: "Ref <ref@customer.test>", to: address.ForwardingAddress, subject: "Something else entirely",
			id: "ref-2@customer.test", references: []string{"unknown-root@elsewhere.test", first},
			inReplyTo: "unknown-parent@elsewhere.test", body: "Following up.",
		}.raw(), "")

		require.Len(t, conversations(tc), 1)
		assert.Equal(t, "references", emailMetadata(t, messages(tc, conversation.ID)[1])["thread_method"])
	})

	t.Run("joins an original that arrives after the reply that referenced it", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, email{
			from: "Max <max@customer.test>", to: address.ForwardingAddress, subject: "Re: Order status",
			id: "late-reply@customer.test", inReplyTo: "late-original@customer.test",
			references: []string{"late-original@customer.test"}, body: "Any news?",
		}.raw(), "")
		deliver(tc, email{
			from: "Max <max@customer.test>", to: address.ForwardingAddress, subject: "Order status",
			id: "late-original@customer.test", body: "Where is my order?",
		}.raw(), "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "reverse_reference", emailMetadata(t, messages(tc, convs[0].ID)[1])["thread_method"])
	})

	t.Run("reopens a recently closed conversation and starts a new one after the window", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "back@customer.test")
		first := *messages(tc, conversation.ID)[0].ChannelMessageID

		closeConversation(tc, conversation)
		deliver(tc, email{
			from: "Back <back@customer.test>", to: address.ForwardingAddress, subject: "Re: Refund for order 1042",
			id: "back-1@customer.test", inReplyTo: first, body: "One more thing.",
		}.raw(), "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "open", convs[0].Status)

		closeConversation(tc, convs[0])
		tc.Clock.Advance(45 * 24 * time.Hour)
		deliver(tc, email{
			from: "Back <back@customer.test>", to: address.ForwardingAddress, subject: "Re: Refund for order 1042",
			id: "back-2@customer.test", inReplyTo: first, body: "Months later.",
		}.raw(), "")

		convs = conversations(tc)
		require.Len(t, convs, 2)
		assert.Equal(t, "back-2@customer.test", *convs[0].ExternalThreadKey)
	})

	t.Run("records an email without a Message-ID once", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		raw := fixture(t, "no_message_id.eml", map[string]string{"TO": address.ForwardingAddress})

		deliver(tc, raw, to(address.ForwardingAddress, "id", "a"))
		deliver(tc, raw, to(address.ForwardingAddress, "id", "b"))

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "Is there a yearly plan?", *messages(tc, convs[0].ID)[0].Body)
		assert.Equal(t, "duplicate", *lastInbound(tc, address).IgnoreReason)
	})

	t.Run("keeps every workspace to its own mail and threads", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "shared@customer.test")
		token := replyToken(tc, address, conversation.ID)

		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		theirID := other.Workspace.ID
		otherAddress := query(tc, theirID, func(ctx context.Context, q *models.Queries) (models.EmailAddress, error) {
			return q.FindDefaultEmailAddress(ctx, theirID)
		})
		otherLocal := otherAddress.ForwardingAddress[:len(otherAddress.ForwardingAddress)-len("@"+inboundDomain)]
		stolen := otherLocal + "+" + token + "@" + inboundDomain

		deliver(tc, email{
			from: "Shared <shared@customer.test>", to: stolen + ", " + address.ForwardingAddress,
			subject: "Re: Refund for order 1042", id: "both@customer.test",
			inReplyTo: *messages(tc, conversation.ID)[0].ChannelMessageID, body: "To both of you.",
		}.raw(), url.Values{"to": {stolen, address.ForwardingAddress}}.Encode())

		assert.Len(t, messages(tc, conversation.ID), 2)

		theirs := inboundEmails(tc, otherAddress)
		require.Len(t, theirs, 1)
		assert.Equal(t, []string{stolen}, theirs[0].EnvelopeTo)
		assert.Equal(t, "new", threadMethod(tc, other.Workspace.ID, *theirs[0].MessageID))
	})

	t.Run("records mail to a recently deleted address without processing it", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		tc.ActingAsOwner().Delete("/api/v1/email-addresses/" + publicID(address)).AssertNoContent()
		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		assert.Empty(t, conversations(tc))
		row := inboundEmails(tc, address)
		require.Len(t, row, 1)
		assert.Equal(t, "address_deleted", *row[0].IgnoreReason)

		tc.Clock.Advance(31 * 24 * time.Hour)
		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}),
			to(address.ForwardingAddress, "id", "later"))
		tc.Table("inbound_emails").AssertCount(1)
	})

	t.Run("holds mail for a suspended workspace as received", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		_, err := tc.Operator().Exec(t.Context(), "UPDATE workspaces SET suspended_at = now() WHERE id = $1",
			address.WorkspaceID)
		require.NoError(t, err)

		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		assert.Empty(t, conversations(tc))
		assert.Equal(t, "received", lastInbound(tc, address).Status)
	})
}

/*
threadMethod returns how a stored message of the workspace was threaded.
*/
func threadMethod(tc *tests.TestCase, workspaceID, messageID ids.UUID) any {
	tc.T.Helper()

	msg := query(tc, workspaceID, func(ctx context.Context, q *models.Queries) (models.Message, error) {
		return q.FindEmailMessage(ctx, models.FindEmailMessageParams{WorkspaceID: workspaceID, ID: messageID})
	})

	return emailMetadata(tc.T, msg)["thread_method"]
}
