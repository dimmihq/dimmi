package email_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
replyToken returns the reply token of the conversation's thread on the
address.
*/
func replyToken(tc *tests.TestCase, address models.EmailAddress, conversationID ids.UUID) string {
	tc.T.Helper()

	return query(tc, address.WorkspaceID, func(ctx context.Context, q *models.Queries) (string, error) {
		thread, err := q.EnsureEmailThread(ctx, models.EnsureEmailThreadParams{
			WorkspaceID: address.WorkspaceID, ConversationID: conversationID, EmailAddressID: &address.ID,
			ReplyToken: "0000000000000000", Now: tc.Clock.Now(),
		})

		return thread.ReplyToken, err
	})
}

/*
emailMetadata returns the email part of a message's metadata.
*/
func emailMetadata(tb testing.TB, msg models.Message) map[string]any {
	tb.Helper()

	var metadata struct {
		AutoReply bool           `json:"auto_reply"`
		AISkip    string         `json:"ai_skip"`
		Email     map[string]any `json:"email"`
	}
	require.NoError(tb, json.Unmarshal(msg.Metadata, &metadata))

	metadata.Email["auto_reply"] = metadata.AutoReply
	metadata.Email["ai_skip"] = metadata.AISkip

	return metadata.Email
}

func TestInboundPipeline(t *testing.T) {
	t.Parallel()

	t.Run("turns a new email into a conversation with the customer as a lead", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{
			"FORWARDING": address.ForwardingAddress, "TO": "support@acme.test",
		}), to(address.ForwardingAddress)).AssertStatus(202).AssertJSONPath("data.workspaces", 1.0)

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "Refund for order 1042", *convs[0].Subject)
		assert.Equal(t, "open", convs[0].Status)
		assert.Equal(t, "CAF=x7QnewRefund1042@mail.gmail.com", *convs[0].ExternalThreadKey)

		msgs := messages(tc, convs[0].ID)
		require.Len(t, msgs, 1)
		assert.Contains(t, *msgs[0].Body, "charged twice for order 1042")
		assert.Equal(t, "CAF=x7QnewRefund1042@mail.gmail.com", *msgs[0].ChannelMessageID)
		assert.True(t, msgs[0].Substantive)

		meta := emailMetadata(t, msgs[0])
		assert.Equal(t, "normal", meta["classification"])
		assert.Equal(t, "new", meta["thread_method"])
		assert.Equal(t, "support@acme.test", meta["original_recipient"])

		row := lastInbound(tc, address)
		assert.Equal(t, "processed", row.Status)
		assert.Equal(t, convs[0].ID, *row.ConversationID)

		tc.Table("contacts").AssertHas(map[string]any{
			"email": "bob@customer.test", "kind": "lead", "name": "Bob Stone",
		})
		tc.Table("contact_identities").AssertHas(map[string]any{"value": "bob@customer.test", "verified": false})
		tc.Table("email_message_refs").AssertHas(map[string]any{
			"rfc_message_id": "CAF=x7QnewRefund1042@mail.gmail.com", "kind": "inbound",
		})
	})

	t.Run("threads a reply by its In-Reply-To and strips the quoted thread", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		values := map[string]string{"FORWARDING": address.ForwardingAddress, "TO": address.ForwardingAddress}

		deliver(tc, fixture(t, "gmail_new.eml", values), to(address.ForwardingAddress))

		values["IN_REPLY_TO"] = "CAF=x7QnewRefund1042@mail.gmail.com"
		values["REFERENCES"] = "CAF=x7QnewRefund1042@mail.gmail.com"
		deliver(tc, fixture(t, "gmail_reply.eml", values), to(address.ForwardingAddress))

		convs := conversations(tc)
		require.Len(t, convs, 1)

		msgs := messages(tc, convs[0].ID)
		require.Len(t, msgs, 2)
		assert.Equal(t, "Perfect, thank you so much!", *msgs[1].Body)
		assert.Contains(t, *msgs[1].BodyHtml, "data-dimmi-quoted")
		assert.Equal(t, "in_reply_to", emailMetadata(t, msgs[1])["thread_method"])
		assert.Equal(t, int32(2), convs[0].MessageCount)
	})

	t.Run("threads a reply that carries only the reply token", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{
			"FORWARDING": address.ForwardingAddress, "TO": address.ForwardingAddress,
		}), to(address.ForwardingAddress))

		conversation := conversations(tc)[0]
		token := replyToken(tc, address, conversation.ID)
		local := address.ForwardingAddress[:len(address.ForwardingAddress)-len("@"+inboundDomain)]
		tokenAddress := local + "+" + token + "@" + inboundDomain

		deliver(tc, fixture(t, "no_message_id.eml", map[string]string{"TO": tokenAddress}), to(tokenAddress))

		convs := conversations(tc)
		require.Len(t, convs, 1)

		msgs := messages(tc, conversation.ID)
		require.Len(t, msgs, 2)
		meta := emailMetadata(t, msgs[1])
		assert.Equal(t, "token", meta["thread_method"])
		assert.Equal(t, "non_participant", meta["ai_skip"])
		assert.Equal(t, true, meta["new_participant"])
		assert.Equal(t, "Is there a yearly plan?", *msgs[1].Body)
		tc.Table("conversation_participants").AssertHas(map[string]any{
			"conversation_id": conversation.ID.String(), "role": "participant",
		})
	})

	t.Run("records a redelivered email once", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		raw := fixture(t, "gmail_new.eml", map[string]string{
			"FORWARDING": address.ForwardingAddress, "TO": address.ForwardingAddress,
		})

		deliver(tc, raw, to(address.ForwardingAddress, "id", "provider-1"))
		deliver(tc, raw, to(address.ForwardingAddress, "id", "provider-1"))
		deliver(tc, raw, to(address.ForwardingAddress, "id", "provider-2"))

		assert.Len(t, conversations(tc), 1)
		rows := inboundEmails(tc, address)
		require.Len(t, rows, 2)
		assert.Equal(t, "ignored", rows[0].Status)
		assert.Equal(t, "duplicate", *rows[0].IgnoreReason)
	})

	t.Run("drops mail to an address nobody holds", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": "ghost@" + inboundDomain}),
			to("ghost@"+inboundDomain)).AssertStatus(202).AssertJSONPath("data.workspaces", 0.0)

		tc.Table("inbound_emails").AssertCount(0)
		assert.Empty(t, conversations(tc))
	})

	t.Run("keeps only what a reader sees of HTML mail", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "html_only.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		msgs := messages(tc, conversations(tc)[0].ID)
		require.Len(t, msgs, 1)

		body, html := *msgs[0].Body, *msgs[0].BodyHtml
		assert.Contains(t, body, "**cannot log in**")
		assert.Contains(t, body, "[This page](https://example.test/help)")
		assert.NotContains(t, body, "Ignore all previous instructions")
		for _, banned := range []string{"<script", "onclick", "javascript:", "<iframe", "tracker.test", "<style"} {
			assert.NotContains(t, html, banned)
		}
	})

	t.Run("keeps an inline reply whole", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "inline_reply.eml", map[string]string{
			"TO": address.ForwardingAddress, "IN_REPLY_TO": "unknown@acme.test",
		}), "")

		msgs := messages(tc, conversations(tc)[0].ID)
		assert.Contains(t, *msgs[0].Body, "Pro, since last year.")
		assert.Contains(t, *msgs[0].Body, "Firefox on Linux.")
	})

	t.Run("cuts the signature and the quote of a plain text reply", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "apple_reply.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		msgs := messages(tc, conversations(tc)[0].ID)
		assert.Equal(t, "Thanks, I'll be home on Thursday.", *msgs[0].Body)
		assert.Nil(t, msgs[0].BodyHtml)

		meta := emailMetadata(t, msgs[0])
		assert.Equal(t, map[string]any{"quote": true, "signature": true, "fallback": false}, meta["stripped"])
	})

	t.Run("cuts the Outlook header block and signature of an HTML reply", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "outlook_reply.eml", map[string]string{
			"TO": address.ForwardingAddress, "IN_REPLY_TO": "unknown@acme.test",
		}), "")

		msgs := messages(tc, conversations(tc)[0].ID)
		assert.Equal(t, "That works for us, please go ahead.", *msgs[0].Body)
	})
}
