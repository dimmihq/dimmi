package email_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
exec runs a statement in the workspace, as the workspace sees its rows.
*/
func exec(tc *tests.TestCase, workspaceID ids.UUID, sql string, args ...any) {
	tc.T.Helper()

	_, err := tc.Operator().Exec(tenancy.With(tc.T.Context(), workspaceID.String()), sql, args...)
	require.NoError(tc.T, err)
}

func TestReportVariants(t *testing.T) {
	t.Parallel()

	t.Run("notes a not-spam report, a delay and a bounce found by In-Reply-To", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer, copied := unique("variant"), unique("copied")
		r := memberReply(tc, startConversation(tc, address, customer), customer)
		r.outbound.CC = []contracts.Recipient{{IdentityType: enums.IdentityTypeEmail, IdentityValue: copied}}
		result, err := send(tc, r)
		require.NoError(t, err)
		require.Len(t, parsed(t, received(t, customer)[0]).Cc, 1)

		values := map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": customer,
			"ORIGINAL": result.ChannelMessageID,
		}

		notSpam := strings.Replace(string(fixture(t, "complaint_arf.eml", values)), "Feedback-Type: abuse",
			"Feedback-Type: not-spam", 1)
		deliver(tc, []byte(notSpam), "")
		assert.Equal(t, "complaint_not_spam", *lastInbound(tc, address).IgnoreReason)

		delayed := strings.Replace(string(fixture(t, "bounce_dsn.eml", values)), "Action: failed", "Action: delayed", 1)
		deliver(tc, []byte(delayed), to(address.ForwardingAddress, "id", "delayed"))
		assert.Equal(t, "bounce", *lastInbound(tc, address).IgnoreReason)
		tc.Table("email_suppressions").AssertCount(0)

		values["RECIPIENT"] = copied
		bare := string(fixture(t, "bounce_dsn.eml", values))
		bare, _, found := strings.Cut(bare, "--dsnboundary\nContent-Type: text/rfc822-headers")
		require.True(t, found)
		bare = strings.Replace(bare+"--dsnboundary--\n", "MIME-Version: 1.0",
			"In-Reply-To: <"+result.ChannelMessageID+">\nMIME-Version: 1.0", 1)
		deliver(tc, []byte(bare), to(address.ForwardingAddress, "id", "bare"))

		assert.Equal(t, "bounce", *lastInbound(tc, address).IgnoreReason)
		tc.Table("email_suppressions").AssertHas(map[string]any{"email": copied})
		tc.Table("messages").AssertHas(map[string]any{"id": r.message.ID.String(), "delivery_status": "pending"})

		heuristic := "From: postmaster@mx.test\r\nTo: " + address.ForwardingAddress +
			"\r\nSubject: Undeliverable: hello\r\n\r\nNo such user."
		deliver(tc, []byte(heuristic), to(address.ForwardingAddress, "id", "heuristic"))
		assert.Equal(t, "bounce_unmatched", *lastInbound(tc, address).IgnoreReason)
	})
}

func TestSenderVariants(t *testing.T) {
	t.Parallel()

	t.Run("throttles and then quarantines a sender who writes too often", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, tests.WithEnv(
			"DIMMI_CHANNELS__EMAIL__INBOUND__SENDER_RATE=1", "DIMMI_CHANNELS__EMAIL__INBOUND__SENDER_QUARANTINE=2",
		))
		address := defaultAddress(tc)

		for i := range 3 {
			deliver(tc, email{
				from: "Fast <fast@customer.test>", to: address.ForwardingAddress, subject: fmt.Sprintf("Note %d", i),
				id: fmt.Sprintf("fast-%d@customer.test", i), body: "Again.",
			}.raw(), "")
		}

		convs := conversations(tc)
		require.Len(t, convs, 3)
		assert.Equal(t, "throttled", emailMetadata(t, messages(tc, convs[1].ID)[0])["ai_skip"])
		assert.True(t, convs[0].Spam)
	})

	t.Run("copies at most fifty people and keeps a believable date", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		cc := make([]string, 0, 55)
		for i := range 55 {
			cc = append(cc, fmt.Sprintf("p%d@customer.test", i))
		}

		raw := email{
			from: "Many <many@customer.test>", to: address.ForwardingAddress, cc: strings.Join(cc, ", "),
			subject: "Big group", id: "many@customer.test", body: "Hi all.",
		}.raw()
		raw = []byte(strings.Replace(string(raw), "Date: Tue, 3 Nov 2026", "Date: Tue, 3 Nov 2099", 1))
		deliver(tc, raw, "")

		tc.Table("conversation_participants").AssertCount(50)
		msg := messages(tc, conversations(tc)[0].ID)[0]
		assert.Equal(t, tc.Clock.Now().UTC(), msg.SentAt.UTC())
	})

	t.Run("reads forwards and form relays without an address", func(t *testing.T) {
		t.Parallel()
		tc, zone := emailCase(t)
		address := defaultAddress(tc)
		owner := tc.Owner.Name + " <" + tc.Owner.Email + ">"

		lost := email{
			from: owner, to: address.ForwardingAddress, subject: "Fwd: Old", id: "lost@example.com",
			body: "---------- Forwarded message ---------\r\nFrom: Bob\r\nSubject: Old\r\n\r\nPlease help.",
		}.raw()
		deliver(tc, signed(t, zone, "example.com", lost), "")

		htmlForward := "From: " + owner + "\r\nTo: " + address.ForwardingAddress + "\r\nSubject: Fwd: Html\r\n" +
			"Message-ID: <html-fwd@example.com>\r\nMIME-Version: 1.0\r\nContent-Type: text/html\r\n\r\n" +
			"<p>See below</p><p>---------- Forwarded message ---------<br>From: Gina &lt;gina@bigco.test&gt;<br>" +
			"Subject: Html</p><p>Broken</p>"
		deliver(tc, signed(t, zone, "example.com", []byte(htmlForward)), "")

		for i, body := range []string{"Message: hello\r\nEmail: visitor@site.test", "Message: no address"} {
			deliver(tc, email{
				from: "Forms <noreply@typeform.com>", to: address.ForwardingAddress, subject: "Form entry",
				id: fmt.Sprintf("form-%d@typeform.com", i), body: body,
			}.raw(), "")
		}

		tc.Table("messages").AssertHas(map[string]any{"type": "note"})
		tc.Table("contacts").AssertHas(map[string]any{"name": "Bob"})
		tc.Table("contacts").AssertHas(map[string]any{"email": "gina@bigco.test"})
		tc.Table("contacts").AssertHas(map[string]any{"email": "visitor@site.test"})
		convs := conversations(tc)
		assert.Equal(t, "no_reply_address", emailMetadata(t, messages(tc, convs[0].ID)[0])["ai_skip"])
	})

	t.Run("stores a repeated inline image once", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		image := "Content-Type: image/png\r\nContent-Transfer-Encoding: base64\r\nContent-ID: <%s>\r\n" +
			"Content-Disposition: inline; filename=\"logo.png\"\r\n\r\n" +
			"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==\r\n"
		raw := "From: Ann <ann@customer.test>\r\nTo: " + address.ForwardingAddress + "\r\nSubject: Logos\r\n" +
			"Message-ID: <logos@customer.test>\r\nMIME-Version: 1.0\r\n" +
			"Content-Type: multipart/related; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: text/html\r\n\r\n<p>Two logos</p>\r\n--b\r\n" + fmt.Sprintf(image, "a@x") +
			"--b\r\n" + fmt.Sprintf(image, "b@x") + "--b--\r\n"

		deliver(tc, []byte(raw), "")

		tc.Table("attachments").AssertCount(1)
	})
}

func TestReplyVariants(t *testing.T) {
	t.Parallel()

	t.Run("threads on the ids the inbox gives when none are filed, and quotes what it can", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer := unique("noname")
		deliver(tc, email{
			from: customer, to: address.ForwardingAddress, subject: "Long story", id: "long." + customer,
			body: strings.Repeat("word ", 600),
		}.raw(), "")
		conversation := conversations(tc)[0]
		exec(tc, conversation.WorkspaceID, "UPDATE workspaces SET timezone = 'Nowhere/Land'")

		r := memberReply(tc, conversation, customer)
		_, err := send(tc, r)
		require.NoError(t, err)

		got := parsed(t, received(t, customer)[0])
		assert.Contains(t, got.Text, ", "+customer+" wrote:")
		assert.Contains(t, got.Text, "…")

		exec(tc, conversation.WorkspaceID, "DELETE FROM email_message_refs")
		exec(tc, conversation.WorkspaceID, "UPDATE messages SET deleted_at = now() WHERE author_type = 'contact'")
		r = memberReply(tc, conversation, customer)
		r.outbound.ExternalThreadKey = ""
		r.outbound.Subject = "Fresh subject"
		r.outbound.ReplyTo = []string{"given@customer.test"}
		_, err = send(tc, r)
		require.NoError(t, err)

		mails := received(t, customer)
		require.Len(t, mails, 2)
		got = parsed(t, mails[0])
		assert.Equal(t, "Fresh subject", got.Subject)
		assert.Equal(t, []string{"given@customer.test"}, got.InReplyTo)
		assert.NotContains(t, got.Text, "wrote:")
		tc.Table("outbound_emails").AssertHas(map[string]any{"kind": "new_conversation"})
	})
}

func TestIngestContract(t *testing.T) {
	t.Parallel()

	t.Run("receives through the inbox contract with or without the channel's hook", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		receiver := tc.App.Actions.ProcessInboundEmail.Receiver
		ctx := tenancy.With(t.Context(), address.WorkspaceID.String())
		missing := ids.New()
		msg := contracts.InboundMessage{
			ConnectionID: address.ChannelConnectionID, DedupeKey: "contract:1", ConversationID: &missing,
			Sender:  contracts.InboundSender{Name: "Nameless"},
			Subject: "Direct", Body: "Hello",
		}

		_, err := receiver.ReceiveInboundMessage(t.Context(), msg)
		require.Error(t, err)
		_, err = receiver.ReceiveInboundMessage(tenancy.With(t.Context(), "not-a-uuid"), msg)
		require.Error(t, err)

		result, err := receiver.ReceiveInboundMessage(ctx, msg)
		require.NoError(t, err)
		assert.True(t, result.ConversationCreated)

		again, err := receiver.ReceiveInboundMessage(ctx, msg)
		require.NoError(t, err)
		assert.True(t, again.Duplicate)

		bad := msg
		bad.DedupeKey, bad.Metadata = "contract:2", map[string]any{"x": make(chan int)}
		_, err = receiver.ReceiveInboundMessage(ctx, bad)
		require.Error(t, err)

		bad.Metadata, bad.Source = nil, map[string]any{"x": make(chan int)}
		_, err = receiver.ReceiveInboundMessage(ctx, bad)
		require.Error(t, err)

		gone := msg
		gone.DedupeKey, gone.ConnectionID = "contract:3", ids.New()
		_, err = receiver.ReceiveInboundMessage(ctx, gone)
		require.Error(t, err)

		failing := msg
		failing.DedupeKey = "contract:4"
		failing.Attachments = []contracts.InboundAttachment{{
			Filename: "x.txt", Open: func(context.Context) (io.ReadCloser, error) { return nil, assert.AnError },
		}}
		_, err = receiver.ReceiveInboundMessage(ctx, failing)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("follows a merged contact to the one it became", func(t *testing.T) {
		t.Parallel()
		tc, zone := emailCase(t)
		address := defaultAddress(tc)
		startConversation(tc, address, "old@customer.test")
		startConversation(tc, address, "new@customer.test")
		ws := address.WorkspaceID
		exec(tc, ws, "UPDATE contacts SET merged_into_id = "+
			"(SELECT id FROM contacts WHERE email = 'new@customer.test') WHERE email = 'old@customer.test'")

		raw := email{
			from: "Old <old@customer.test>", to: address.ForwardingAddress, subject: "Merged",
			id: "merged@customer.test", body: "Still me.",
		}.raw()
		deliver(tc, signed(t, zone, "customer.test", raw), "")

		msgs := query(tc, ws, func(ctx context.Context, q *models.Queries) (models.Contact, error) {
			return q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
				WorkspaceID: ws, Type: "email", Value: "new@customer.test",
			})
		})
		tc.Table("messages").AssertHas(map[string]any{"author_id": msgs.ID.String(), "body": "Still me."})
	})

	t.Run("leaves mail alone that has no address left", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}), "")
		row := lastInbound(tc, address)

		exec(tc, row.WorkspaceID, "UPDATE inbound_emails SET status = 'received', email_address_id = NULL")
		require.NoError(t, tc.App.Actions.ProcessInboundEmail.Handle(t.Context(), row.WorkspaceID, row.ID))

		exec(tc, row.WorkspaceID, "UPDATE inbound_emails SET email_address_id = $1, raw_path = NULL", address.ID)
		require.Error(t, tc.App.Actions.ProcessInboundEmail.Handle(t.Context(), row.WorkspaceID, row.ID))
	})

	t.Run("joins a known thread key and lets the original contact be copied", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		receiver := tc.App.Actions.ProcessInboundEmail.Receiver
		ctx := tenancy.With(t.Context(), address.WorkspaceID.String())
		person := func(address string) contracts.InboundSender {
			return contracts.InboundSender{
				Email: address, Identities: []contracts.IdentityClaim{{Type: "email", Value: address}},
			}
		}
		bob, ana := person("bob@customer.test"), person("ana@customer.test")

		first, err := receiver.ReceiveInboundMessage(ctx, contracts.InboundMessage{
			ConnectionID: address.ChannelConnectionID, DedupeKey: "key:1", ExternalThreadKey: "k1", Sender: bob,
			Body: "Hi",
		})
		require.NoError(t, err)

		second, err := receiver.ReceiveInboundMessage(ctx, contracts.InboundMessage{
			ConnectionID: address.ChannelConnectionID, DedupeKey: "key:2", ExternalThreadKey: "k1", Sender: ana,
			Participants: []contracts.InboundSender{bob}, Body: "Me too",
		})
		require.NoError(t, err)
		assert.Equal(t, first.ConversationID, second.ConversationID)
		tc.Table("conversation_participants").AssertCount(1)
	})
}

func TestMoreReplyBranches(t *testing.T) {
	t.Parallel()

	t.Run("retries a reply that failed in passing and refuses one the mailer rejects", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, tests.WithEnv(
			"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=smtp", "DIMMI_MAIL__MAILERS__CUSTOMER__HOST=127.0.0.1",
			"DIMMI_MAIL__MAILERS__CUSTOMER__PORT=1", "DIMMI_MAIL__MAILERS__CUSTOMER__ALLOW_INSECURE=true",
		))
		address := defaultAddress(tc)
		customer := unique("retry")
		conversation := startConversation(tc, address, customer)
		r := memberReply(tc, conversation, customer)

		_, err := send(tc, r)
		require.Error(t, err)
		_, err = send(tc, r)
		require.Error(t, err)
		tc.Table("outbound_emails").AssertHas(map[string]any{"attempts": 2})

		bad := memberReply(tc, conversation, customer)
		bad.outbound.Subject = "Broken\nsubject"
		_, err = send(tc, bad)
		require.ErrorIs(t, err, contracts.ErrPermanent)
		tc.Table("outbound_emails").AssertHas(map[string]any{"status": "failed"})
	})

	t.Run("sends from the default address once the conversation's address is gone", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		sales := tc.ActingAsOwner().PostJSON("/api/v1/email-addresses", map[string]any{"address": "sales@acme.test"}).
			AssertCreated()
		customer := unique("moved")
		deliver(tc, email{
			from: customer, to: sales.String("data.forwarding_address"), subject: "Sales question",
			id: "sales." + customer, body: "Price?", references: []string{"sales." + customer},
		}.raw(), "")
		conversation := conversations(tc)[0]

		tc.ActingAsOwner().Delete("/api/v1/email-addresses/" + sales.String("data.id")).AssertNoContent()
		result, err := send(tc, memberReply(tc, conversation, customer))
		require.NoError(t, err)
		assert.Equal(t, address.ForwardingAddress, result.Metadata["from"])
	})

	t.Run("picks the open conversation among several sharing an id", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		first := startConversation(tc, address, "one@customer.test")
		third := startConversation(tc, address, "three@customer.test")
		tc.Clock.Advance(time.Minute)
		second := startConversation(tc, address, "two@customer.test")
		ws := address.WorkspaceID
		exec(tc, ws, "INSERT INTO email_message_refs (workspace_id, rfc_message_id, conversation_id, kind) "+
			"VALUES ($1, 'shared@x.test', $2, 'referenced'), ($1, 'shared@x.test', $3, 'referenced'), "+
			"($1, 'shared@x.test', $4, 'referenced')", ws, first.ID, second.ID, third.ID)
		closeConversation(tc, first)

		deliver(tc, email{
			from: "One <one@customer.test>", to: address.ForwardingAddress, subject: "Re: shared",
			id: "shared-reply@customer.test", inReplyTo: "shared@x.test", body: "Which one?",
		}.raw(), "")

		assert.Len(t, messages(tc, second.ID), 2)
	})

	t.Run("routes a token address after the plain one and records long diagnostics", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		conversation := startConversation(tc, address, "token@customer.test")
		token := replyToken(tc, address, conversation.ID)
		local := address.ForwardingAddress[:len(address.ForwardingAddress)-len("@"+inboundDomain)]
		tokenAddress := local + "+" + token + "@" + inboundDomain

		deliver(tc, email{
			from: "Token <token@customer.test>", to: address.ForwardingAddress, subject: "Re: Refund for order 1042",
			id: "both@customer.test", body: "Both.",
		}.raw(), "to="+address.ForwardingAddress+"&to="+strings.ReplaceAll(tokenAddress, "+", "%2B")+
			"&to="+address.ForwardingAddress)
		assert.Len(t, messages(tc, conversation.ID), 2)

		long := strings.Replace(string(fixture(t, "bounce_dsn.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress, "RECIPIENT": "x@y.test",
			"ORIGINAL": "",
		})), "does not exist.", strings.Repeat("very ", 100)+"long.", 1)
		long = strings.Replace(long, "Message-ID: <>\n", "", 1)
		deliver(tc, []byte(long), to(address.ForwardingAddress, "id", "long"))
		assert.Equal(t, "bounce_unmatched", *lastInbound(tc, address).IgnoreReason)

		result, err := send(tc, memberReply(tc, conversation, "token@customer.test"))
		require.NoError(t, err)
		matched := strings.Replace(string(fixture(t, "bounce_dsn.eml", map[string]string{
			"TO": address.ForwardingAddress, "FORWARDING": address.ForwardingAddress,
			"RECIPIENT": "token@customer.test", "ORIGINAL": result.ChannelMessageID,
		})), "does not exist.", strings.Repeat("very ", 100)+"long.", 1)
		deliver(tc, []byte(matched), to(address.ForwardingAddress, "id", "matched"))
		assert.Equal(t, "bounce", *lastInbound(tc, address).IgnoreReason)
	})
}
