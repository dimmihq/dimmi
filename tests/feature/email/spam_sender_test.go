package email_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
markSpam sets the conversation aside as spam, the way a teammate does.
*/
func markSpam(tc *tests.TestCase, conversation models.Conversation) {
	tc.T.Helper()

	query(tc, conversation.WorkspaceID, func(ctx context.Context, q *models.Queries) (models.Conversation, error) {
		return q.SetConversationSpam(ctx, models.SetConversationSpamParams{
			Spam: true, Now: tc.Clock.Now(), WorkspaceID: conversation.WorkspaceID, ID: conversation.ID,
		})
	})
}

/*
answer delivers the customer's reply to the first message of the
conversation, with the extra delivery parameters given.
*/
func answer(tc *tests.TestCase, conversation models.Conversation, id string, extra ...string) {
	tc.T.Helper()

	address := defaultAddress(tc)
	first := *messages(tc, conversation.ID)[0].ChannelMessageID
	deliver(tc, email{
		from: "Sam <sam@customer.test>", to: address.ForwardingAddress, subject: "Re: Refund for order 1042",
		id: id + "@customer.test", inReplyTo: first, body: "Still waiting.",
	}.raw(), to(address.ForwardingAddress, extra...))
}

func TestSpamFollowsTheSender(t *testing.T) {
	t.Parallel()

	t.Run("keeps a blocked customer's reply from reopening their thread", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "sam@customer.test")
		closeConversation(tc, conversation)
		_, err := tc.Operator().Exec(t.Context(), `UPDATE contacts SET blocked_at = now() WHERE id = $1`,
			conversation.ContactID)
		require.NoError(t, err)

		answer(tc, conversation, "blocked-1")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "closed", convs[0].Status)
		assert.Len(t, messages(tc, conversation.ID), 2)
	})

	t.Run("keeps a reply the provider calls spam from reopening the thread", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "sam@customer.test")
		closeConversation(tc, conversation)

		answer(tc, conversation, "verdict-1", "spam", "fail")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "closed", convs[0].Status)
	})

	t.Run("sets aside a new thread when the old one was spam", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "sam@customer.test")
		markSpam(tc, conversation)
		closeConversation(tc, conversation)
		tc.Clock.Advance(45 * 24 * time.Hour)

		answer(tc, conversation, "late-1")

		convs := conversations(tc)
		require.Len(t, convs, 2)
		assert.True(t, convs[0].Spam)
	})

	t.Run("sets aside fresh mail from a sender whose last conversation is spam", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		markSpam(tc, startConversation(tc, address, "sam@customer.test"))

		deliver(tc, email{
			from: "Sam <sam@customer.test>", to: address.ForwardingAddress, subject: "Great offer",
			id: "fresh-1@customer.test", body: "Buy now.",
		}.raw(), to(address.ForwardingAddress))

		convs := conversations(tc)
		require.Len(t, convs, 2)
		assert.True(t, convs[0].Spam)
	})
}
