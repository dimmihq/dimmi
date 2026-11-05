package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailSystemNotices(t *testing.T) {
	t.Parallel()

	t.Run("fails the reply a mail system's notice quotes without stopping mail to the address", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("notice")
		r := memberReply(tc, startConversation(tc, address, customer), customer)
		result, err := send(tc, r)
		require.NoError(t, err)

		notice := "From: Mail Delivery System <MAILER-DAEMON@mx.example.com>\r\nTo: " + address.ForwardingAddress +
			"\r\nSubject: Message not delivered\r\nX-Failed-Recipients: " + customer +
			"\r\nContent-Type: text/plain\r\n\r\nYour message could not be delivered.\r\n\r\n" +
			"------ Original message ------\r\nMessage-ID: <" + result.ChannelMessageID + ">\r\nSubject: Re: Help\r\n"
		deliver(tc, []byte(notice), "")

		assert.Equal(t, "bounce", *lastInbound(tc, address).IgnoreReason)
		tc.Table("messages").AssertHas(map[string]any{"id": r.message.ID.String(), "delivery_status": "failed"})
		tc.Table("email_suppressions").AssertCount(0)
		assert.Len(t, conversations(tc), 1)
	})

	t.Run("reads a postmaster's notice in another language as a bounce, never a conversation", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, email{
			from: "postmaster@mx.example.com", to: address.ForwardingAddress, subject: "Nicht zustellbar: Hallo",
			id: "notice-de@mx.example.com", body: "Die Nachricht konnte nicht zugestellt werden.",
		}.raw(), "")

		assert.Empty(t, conversations(tc))
		assert.Equal(t, "bounce_unmatched", *lastInbound(tc, address).IgnoreReason)
	})

	t.Run("keeps a read receipt from reopening a closed conversation", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "bob@customer.test")
		first := messages(tc, conversation.ID)[0]
		closeConversation(tc, conversation)

		receipt := "From: Bob <bob@customer.test>\r\nTo: " + address.ForwardingAddress +
			"\r\nSubject: Read: Help\r\nMessage-ID: <read-1@customer.test>\r\nIn-Reply-To: <" +
			*first.ChannelMessageID + ">\r\nMIME-Version: 1.0\r\n" +
			"Content-Type: multipart/report; report-type=disposition-notification; boundary=r\r\n\r\n" +
			"--r\r\nContent-Type: text/plain\r\n\r\nYour message was read.\r\n" +
			"--r\r\nContent-Type: message/disposition-notification\r\n\r\n" +
			"Final-Recipient: rfc822; bob@customer.test\r\n" +
			"Disposition: manual-action/MDN-sent-manually; displayed\r\n" +
			"--r--\r\n"
		deliver(tc, []byte(receipt), "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "closed", convs[0].Status)
		msgs := messages(tc, conversation.ID)
		assert.False(t, msgs[len(msgs)-1].Substantive)
	})
}
