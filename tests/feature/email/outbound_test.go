package email_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

func TestOutboundReplies(t *testing.T) {
	t.Parallel()

	t.Run("sends a reply that threads in the customer's mailbox and brings the answer back", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("bob")
		conversation := startConversation(tc, address, customer)
		first := messages(tc, conversation.ID)[0]

		result, err := send(tc, memberReply(tc, conversation, customer))
		require.NoError(t, err)
		assert.Equal(t, enums.DeliveryStatusSent, result.Status)
		assert.Empty(t, result.ExternalThreadKey)

		mails := received(t, customer)
		require.Len(t, mails, 1)
		got := parsed(t, mails[0])

		assert.Equal(t, result.ChannelMessageID, got.MessageID)
		assert.True(t, strings.HasPrefix(got.MessageID, models.PublicID(models.PrefixMessage, first.ID)[:4]))
		assert.Equal(t, []string{*first.ChannelMessageID}, got.InReplyTo)
		assert.Contains(t, got.References, *first.ChannelMessageID)
		assert.Equal(t, "Re: Refund for order 1042", got.Subject)
		assert.Equal(t, address.ForwardingAddress, got.From.Email)
		assert.Equal(t, "Sam from "+address.DisplayName, got.From.Name)
		require.Len(t, got.ReplyTo, 1)
		assert.Contains(t, got.ReplyTo[0].Email, "+")
		assert.True(t, strings.HasSuffix(got.ReplyTo[0].Email, "@"+inboundDomain))
		assert.NotEmpty(t, got.Header.Get("X-Dimmi-Loop"))
		assert.Empty(t, got.Header.Get("Auto-Submitted"))
		assert.Contains(t, got.Text, "the refund is on its way")
		assert.Contains(t, got.Text, "> I was charged twice.")

		deliver(tc, email{
			from: "Bob Stone <" + customer + ">", to: got.ReplyTo[0].Email, subject: "Re: Refund for order 1042",
			id: "answer." + customer, inReplyTo: got.MessageID, references: got.References,
			body: "Great, thanks!\r\n\r\nOn Tue, Nov 3, 2026 at 10:00 AM Sam wrote:\r\n> Hi, the refund is on its way.",
		}.raw(), to(got.ReplyTo[0].Email))

		convs := conversations(tc)
		require.Len(t, convs, 1)
		msgs := messages(tc, conversation.ID)
		require.Len(t, msgs, 3)
		assert.Equal(t, "Great, thanks!", *msgs[2].Body)
	})

	t.Run("recognizes its own reply coming back and ignores it", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("loop")
		conversation := startConversation(tc, address, customer)

		_, err := send(tc, memberReply(tc, conversation, customer))
		require.NoError(t, err)

		deliver(tc, received(t, customer)[0], to(address.ForwardingAddress))

		assert.Len(t, messages(tc, conversation.ID), 2)
		row := lastInbound(tc, address)
		assert.Equal(t, "loop", *row.Classification)
		assert.Equal(t, "loop_own_outbound", *row.IgnoreReason)
	})

	t.Run("never sends the same reply twice", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("twice")
		r := memberReply(tc, startConversation(tc, address, customer), customer)

		first, err := send(tc, r)
		require.NoError(t, err)
		again, err := send(tc, r)
		require.NoError(t, err)

		assert.Equal(t, first.ChannelMessageID, again.ChannelMessageID)
		assert.Len(t, received(t, customer), 1)
		tc.Table("outbound_emails").AssertCount(1, map[string]any{"status": "sent", "transport": "smtp"})
	})

	t.Run("fails a reply to a contact without an address", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		r := memberReply(tc, startConversation(tc, address, unique("none")), "")

		_, err := send(tc, r)

		var sendErr *contracts.SendError
		require.ErrorAs(t, err, &sendErr)
		assert.Equal(t, "recipient_missing", sendErr.Code)
		require.ErrorIs(t, err, contracts.ErrRecipientUnreachable)
	})

	t.Run("bounces stop mail to the address and fail the reply", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		customer := unique("gone")
		conversation := startConversation(tc, address, customer)
		r := memberReply(tc, conversation, customer)

		result, err := send(tc, r)
		require.NoError(t, err)

		deliver(tc, fixture(t, "bounce_dsn.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": customer,
			"ORIGINAL": result.ChannelMessageID,
		}), "")

		row := lastInbound(tc, address)
		assert.Equal(t, "bounce", *row.Classification)
		assert.Equal(t, "bounce", *row.IgnoreReason)
		tc.Table("email_suppressions").AssertHas(map[string]any{
			"email": customer, "reason": "hard_bounce", "scope": "all",
		})
		tc.Table("outbound_emails").AssertHas(map[string]any{"status": "bounced"})
		tc.Table("messages").AssertHas(map[string]any{"id": r.message.ID.String(), "delivery_status": "failed"})

		_, err = send(tc, memberReply(tc, conversation, customer))

		var sendErr *contracts.SendError
		require.ErrorAs(t, err, &sendErr)
		assert.Equal(t, "recipient_suppressed", sendErr.Code)
	})

	t.Run("complaints stop all mail to the complainer", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		customer := unique("angry")
		result, err := send(tc, memberReply(tc, startConversation(tc, address, customer), customer))
		require.NoError(t, err)

		deliver(tc, fixture(t, "complaint_arf.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": customer,
			"ORIGINAL": result.ChannelMessageID,
		}), "")

		assert.Equal(t, "complaint", *lastInbound(tc, address).IgnoreReason)
		tc.Table("email_suppressions").AssertHas(map[string]any{"email": customer, "reason": "complaint"})
		tc.Table("outbound_emails").AssertHas(map[string]any{"status": "complained"})
	})

	t.Run("records reports about mail it never sent as unmatched", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		values := map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": "spoofer@elsewhere.test", "RECIPIENT": "victim@else.test",
			"ORIGINAL": "never-sent@elsewhere.test",
		}

		deliver(tc, fixture(t, "bounce_dsn.eml", values), "")
		assert.Equal(t, "bounce_unmatched", *lastInbound(tc, address).IgnoreReason)

		deliver(tc, fixture(t, "complaint_arf.eml", values), "")
		assert.Equal(t, "complaint_unmatched", *lastInbound(tc, address).IgnoreReason)
		tc.Table("email_suppressions").AssertCount(0)
	})
}
