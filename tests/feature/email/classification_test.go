package email_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestClassification(t *testing.T) {
	t.Parallel()

	t.Run("appends an out-of-office reply quietly and never starts a thread with one", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "bob@customer.test")
		first := messages(tc, conversation.ID)[0]

		deliver(tc, fixture(t, "auto_reply.eml", map[string]string{
			"TO": address.ForwardingAddress, "IN_REPLY_TO": *first.ChannelMessageID,
		}), "")

		msgs := messages(tc, conversation.ID)
		require.Len(t, msgs, 2)
		assert.False(t, msgs[1].Substantive)
		assert.Equal(t, true, emailMetadata(t, msgs[1])["auto_reply"])
		assert.Equal(t, "auto_reply", emailMetadata(t, msgs[1])["classification"])

		deliver(tc, fixture(t, "auto_reply.eml", map[string]string{
			"TO": address.ForwardingAddress, "IN_REPLY_TO": "unknown@elsewhere.test",
		}), to(address.ForwardingAddress, "id", "second-ooo"))

		assert.Len(t, conversations(tc), 1)
		assert.Equal(t, "auto_reply_no_thread", *lastInbound(tc, address).IgnoreReason)
	})

	t.Run("opens bulk mail as a closed conversation, or drops it when the address says so", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		raw := fixture(t, "newsletter.eml", map[string]string{"TO": address.ForwardingAddress})

		deliver(tc, raw, "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "closed", convs[0].Status)
		assert.False(t, messages(tc, convs[0].ID)[0].Substantive)

		tc.ActingAsOwner().PatchJSON("/api/v1/email-addresses/"+publicID(address),
			map[string]any{"settings": map[string]any{"bulk_mail_handling": "drop"}}).AssertOK()
		deliver(tc, raw, to(address.ForwardingAddress, "id", "again"))

		assert.Len(t, conversations(tc), 1)
		assert.Equal(t, "bulk_drop", *lastInbound(tc, address).IgnoreReason)
	})

	t.Run("hands a teammate's forward over to the original sender with a note", func(t *testing.T) {
		t.Parallel()
		tc, zone := emailCase(t)
		address := defaultAddress(tc)
		raw := fixture(t, "forwarded_by_member.eml", map[string]string{
			"TO": address.ForwardingAddress, "MEMBER": tc.Owner.Email, "MEMBER_NAME": tc.Owner.Name,
		})

		deliver(tc, signed(t, zone, "example.com", raw), "")

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.Equal(t, "Broken export", *convs[0].Subject)

		msgs := messages(tc, convs[0].ID)
		require.Len(t, msgs, 2)
		assert.Contains(t, *msgs[0].Body, "the CSV export has been failing")
		assert.Equal(t, "forwarded", emailMetadata(t, msgs[0])["classification"])
		assert.Equal(t, "note", msgs[1].Type)
		assert.Equal(t, "Forwarded by "+tc.Owner.Name+": Can someone take this one? He is on the enterprise plan.",
			*msgs[1].Body)
		tc.Table("contacts").AssertHas(map[string]any{"email": "gina@bigco.test", "email_verified": false})
	})

	t.Run("marks a teammate's own test mail as a test", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, email{
			from: tc.Owner.Name + " <" + tc.Owner.Email + ">", to: address.ForwardingAddress, subject: "Testing",
			id: "member-test@example.com", body: "Does this arrive?",
		}.raw(), to(address.ForwardingAddress, "spf", "pass", "from", tc.Owner.Email))

		msgs := messages(tc, conversations(tc)[0].ID)
		assert.Equal(t, "test", emailMetadata(t, msgs[0])["classification"])
		tc.Table("contact_identities").AssertHas(map[string]any{"value": tc.Owner.Email, "verified": true})
	})

	t.Run("keeps a forwarding confirmation for the admins and starts no conversation", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "gmail_confirmation.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress,
		}), "")

		assert.Empty(t, conversations(tc))
		assert.Equal(t, "forwarding_confirmation", *lastInbound(tc, address).IgnoreReason)

		res := tc.ActingAsOwner().GetJSON("/api/v1/email-addresses/" + publicID(address)).AssertOK()
		res.AssertJSONPath("data.forwarding_confirmation.code", "428715637")
		res.AssertJSONPath("data.forwarding_confirmation.provider", "gmail")
		assert.Contains(t, res.String("data.forwarding_confirmation.confirm_url"), "https://mail-settings.google.com/")
		assert.NotEmpty(t, res.String("data.inbound_verified_at"))
	})

	t.Run("sets spam aside", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}),
			to(address.ForwardingAddress, "spam", "fail"))

		convs := conversations(tc)
		require.Len(t, convs, 1)
		assert.True(t, convs[0].Spam)
		assert.Equal(t, "quarantined", lastInbound(tc, address).Status)
	})

	t.Run("takes the visitor behind a form service as the customer", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "form_relay.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		msgs := messages(tc, conversations(tc)[0].ID)
		assert.Equal(t, "normal", emailMetadata(t, msgs[0])["classification"])
		assert.Equal(t, "webflow.com", emailMetadata(t, msgs[0])["form_relay"])
		tc.Table("contacts").AssertHas(map[string]any{"email": "jules@visitor.test", "name": "Jules Verne"})
		tc.Table("contacts").AssertMissing(map[string]any{"email": "no-reply@webflow.com"})
	})

	t.Run("never answers a spoofed verified customer automatically", func(t *testing.T) {
		t.Parallel()
		tc, zone := emailCase(t)
		address := defaultAddress(tc)
		raw := email{
			from: "Bob <bob@customer.test>", to: address.ForwardingAddress, subject: "Hello there friends",
			id: "signed-1@customer.test", body: "Signed and proven.",
		}.raw()

		deliver(tc, signed(t, zone, "customer.test", raw), "")
		tc.Table("contacts").AssertHas(map[string]any{"email": "bob@customer.test", "email_verified": true})

		deliver(tc, email{
			from: "Bob <bob@customer.test>", to: address.ForwardingAddress, subject: "Change my bank details",
			id: "spoofed-1@attacker.test", body: "Send the refund elsewhere.",
		}.raw(), "")

		convs := conversations(tc)
		require.Len(t, convs, 2)
		assert.Equal(t, "unverified_sender", emailMetadata(t, messages(tc, convs[0].ID)[0])["ai_skip"])
	})

	t.Run("stores files and lists those it skipped", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, fixture(t, "attachments.eml", map[string]string{"TO": address.ForwardingAddress}), "")

		msg := messages(tc, conversations(tc)[0].ID)[0]
		tc.Table("attachments").AssertCount(2, map[string]any{"message_id": msg.ID.String()})
		tc.Table("attachments").AssertHas(map[string]any{"filename": "invoice.pdf", "inline": false})
		tc.Table("attachments").AssertHas(map[string]any{
			"filename": "screen.png", "inline": true, "content_id": "screen1@customer.test",
		})

		skipped, err := json.Marshal(emailMetadata(t, msg)["skipped_attachments"])
		require.NoError(t, err)
		assert.JSONEq(t, `[{"filename":"setup.exe","size":32,"reason":"blocked_type"}]`, string(skipped))
		tc.Table("conversation_participants").AssertHas(map[string]any{"role": "cc"})
		tc.Table("contacts").AssertHas(map[string]any{"email": "ivan@customer.test"})
	})
}

/*
publicID returns the address's public id.
*/
func publicID(address models.EmailAddress) string {
	return models.PublicID(models.PrefixEmailAddress, address.ID)
}

/*
closeConversation closes the conversation as if a teammate did when
it was.
*/
func closeConversation(tc *tests.TestCase, conversation models.Conversation) {
	tc.T.Helper()

	query(tc, conversation.WorkspaceID, func(ctx context.Context, q *models.Queries) (struct{}, error) {
		return struct{}{}, q.CloseEmailConversation(ctx, models.CloseEmailConversationParams{
			Now: tc.Clock.Now(), WorkspaceID: conversation.WorkspaceID, ID: conversation.ID,
		})
	})
}
