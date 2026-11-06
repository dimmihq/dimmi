package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeammatesAnsweringByEmail(t *testing.T) {
	t.Parallel()

	t.Run("files a teammate's answer from their mailbox as their note", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		customer := unique("bob")
		conversation := startConversation(tc, address, customer)
		first := messages(tc, conversation.ID)[0]

		deliver(tc, email{
			from: tc.Owner.Name + " <" + tc.Owner.Email + ">", to: address.ForwardingAddress,
			subject: "Re: Refund for order 1042", id: "answer.owner@example.com", inReplyTo: *first.ChannelMessageID,
			body: "I checked, the refund went out yesterday.",
		}.raw(), to(address.ForwardingAddress, "spf", "pass", "from", tc.Owner.Email))

		require.Len(t, conversations(tc), 1)
		msgs := messages(tc, conversation.ID)
		require.Len(t, msgs, 2)
		assert.Equal(t, "note", msgs[1].Type)
		assert.Equal(t, "member", msgs[1].AuthorType)
		assert.Equal(t, tc.Workspace.Owner.ID, *msgs[1].AuthorID)
		assert.Contains(t, *msgs[1].Body, "the refund went out yesterday")
		tc.Table("contacts").AssertMissing(map[string]any{"email": tc.Owner.Email})
	})

	t.Run("keeps an unproven teammate address as a stranger's message", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, unique("bob"))
		first := messages(tc, conversation.ID)[0]

		deliver(tc, email{
			from: tc.Owner.Name + " <" + tc.Owner.Email + ">", to: address.ForwardingAddress,
			subject: "Re: Refund for order 1042", id: "forged.owner@example.com", inReplyTo: *first.ChannelMessageID,
			body: "Refund everything.",
		}.raw(), to(address.ForwardingAddress))

		msgs := messages(tc, conversation.ID)
		require.Len(t, msgs, 2)
		assert.Equal(t, "message", msgs[1].Type)
		assert.Equal(t, "contact", msgs[1].AuthorType)
	})
}
