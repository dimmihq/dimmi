package email_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestReplyOptions(t *testing.T) {
	t.Parallel()

	t.Run("sends to the recipients and from the address the request names", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		address := defaultAddress(tc)
		customer, copied, other := unique("opt"), unique("copy"), unique("other")
		r := memberReply(tc, startConversation(tc, address, customer), customer)

		sales := tc.ActingAsOwner().PostJSON("/api/v1/email-addresses", map[string]any{
			"address": unique("sales"), "display_name": "Acme Sales",
		}).AssertCreated()
		tc.ActingAsOwner().PatchJSON("/api/v1/email-addresses/"+sales.String("data.id"), map[string]any{
			"from_name_style": "name_only", "reply_to_mode": "none",
			"settings": map[string]any{"quote_previous": false},
		}).AssertOK()

		r.outbound.Options = []byte(`{"email":{"to":["` + other + `","` + address.ForwardingAddress + `"],"cc":["` +
			copied + `","not an address"],"from_address_id":"` + sales.String("data.id") + `"}}`)
		r.outbound.Author = contracts.OutboundAuthor{Type: enums.AuthorTypeAgent, Name: "Robo"}

		_, err := send(tc, r)
		require.NoError(t, err)

		got := parsed(t, received(t, other)[0])
		assert.Equal(t, sales.String("data.forwarding_address"), got.From.Email)
		assert.Equal(t, "Acme Sales", got.From.Name)
		assert.Empty(t, got.ReplyTo)
		assert.Equal(t, "auto-replied", got.Header.Get("Auto-Submitted"))
		assert.NotContains(t, got.Text, "> I was charged twice.")
		require.Len(t, got.Cc, 1)
		assert.Equal(t, copied, got.Cc[0].Email)
		assert.Empty(t, received(t, customer))
		tc.Table("outbound_emails").AssertHas(map[string]any{"kind": "ai_reply"})
	})

	t.Run("refuses options it cannot read and falls back to the default address", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		customer := unique("bad")
		conversation := startConversation(tc, address, customer)

		r := memberReply(tc, conversation, customer)
		r.outbound.Options = []byte(`not json`)
		_, err := send(tc, r)
		require.ErrorIs(t, err, contracts.ErrPermanent)

		r = memberReply(tc, conversation, customer)
		r.outbound.Options = []byte(`{"from_address_id":"ema_bogus","to":["` + customer + `"]}`)
		_, err = send(tc, r)
		require.ErrorIs(t, err, contracts.ErrPermanent)

		r = memberReply(tc, conversation, customer)
		r.outbound.ExternalThreadKey = ""
		r.outbound.Author = contracts.OutboundAuthor{Type: enums.AuthorTypeSystem, Name: "Acme"}
		result, err := send(tc, r)
		require.NoError(t, err)
		assert.Equal(t, result.ChannelMessageID, result.ExternalThreadKey)
		tc.Table("outbound_emails").AssertHas(map[string]any{"kind": "system"})

		tc.ActingAsOwner().Delete("/api/v1/email-addresses/" + publicID(address)).AssertNoContent()
		_, err = send(tc, memberReply(tc, conversation, customer))

		var sendErr *contracts.SendError
		require.ErrorAs(t, err, &sendErr)
		assert.Equal(t, "no_sender_available", sendErr.Code)
	})

	t.Run("reports a mail server that does not answer as a passing failure", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, tests.WithEnv(
			"DIMMI_MAIL__MAILERS__CUSTOMER__DRIVER=smtp", "DIMMI_MAIL__MAILERS__CUSTOMER__HOST=127.0.0.1",
			"DIMMI_MAIL__MAILERS__CUSTOMER__PORT=1", "DIMMI_MAIL__MAILERS__CUSTOMER__ALLOW_INSECURE=true",
		))
		address := defaultAddress(tc)
		customer := unique("down")

		_, err := send(tc, memberReply(tc, startConversation(tc, address, customer), customer))

		var sendErr *contracts.SendError
		require.ErrorAs(t, err, &sendErr)
		assert.Equal(t, "provider_unavailable", sendErr.Code)
		require.NoError(t, sendErr.Unwrap())
		assert.NotEmpty(t, sendErr.Error())
		tc.Table("outbound_emails").AssertHas(map[string]any{"status": "queued"})

		tc.ActingAsOwner().PostJSON("/api/v1/email-addresses/"+publicID(address)+"/tests", nil).AssertStatus(503)
		tc.Table("outbound_emails").AssertHas(map[string]any{"kind": "test", "status": "failed"})
	})
}

func TestInboundEdges(t *testing.T) {
	t.Parallel()

	t.Run("ignores mail that names no sender", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		raw := []byte("Subject: hi\r\nTo: " + address.ForwardingAddress + "\r\n\r\nhello")
		deliver(tc, raw, to(address.ForwardingAddress))

		assert.Equal(t, "unparseable", *lastInbound(tc, address).IgnoreReason)
	})

	t.Run("lets a copied participant answer without flagging them", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		deliver(tc, email{
			from: "Bob <bob@customer.test>", to: address.ForwardingAddress, cc: "Ana <ana@customer.test>",
			subject: "Team question", id: "team-1@customer.test", body: "Question for you.",
		}.raw(), "")
		deliver(tc, email{
			from: "Ana <ana@customer.test>", to: address.ForwardingAddress, subject: "Re: Team question",
			id: "team-2@customer.test", inReplyTo: "team-1@customer.test", body: "Adding to that.",
		}.raw(), "")

		msgs := messages(tc, conversations(tc)[0].ID)
		require.Len(t, msgs, 2)
		assert.Equal(t, "", emailMetadata(t, msgs[1])["ai_skip"])
	})

	t.Run("lists the files beyond the count and size limits as skipped", func(t *testing.T) {
		t.Parallel()
		for limit, want := range map[string][]string{
			"40":  {"too_large", "blocked_type", "too_large"},
			"100": {"too_many", "too_many"},
		} {
			tc, _ := emailCase(t, tests.WithEnv(
				"DIMMI_CHANNELS__EMAIL__INBOUND__MAX_ATTACHMENTS=1",
				"DIMMI_CHANNELS__EMAIL__INBOUND__MAX_ATTACHMENT_BYTES="+limit,
			))
			address := defaultAddress(tc)

			deliver(tc, fixture(t, "attachments.eml", map[string]string{"TO": address.ForwardingAddress}), "")

			var skipped []struct{ Reason string }

			meta := emailMetadata(t, messages(tc, conversations(tc)[0].ID)[0])
			encoded, err := json.Marshal(meta["skipped_attachments"])
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(encoded, &skipped))

			reasons := []string{}
			for _, s := range skipped {
				reasons = append(reasons, s.Reason)
			}

			assert.Equal(t, want, reasons, limit)
		}
	})

	t.Run("records a failure when the raw message is gone, and leaves finished mail alone", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		deliver(tc, fixture(t, "gmail_new.eml", map[string]string{"TO": address.ForwardingAddress}), "")
		row := lastInbound(tc, address)

		require.NoError(t, tc.App.Actions.ProcessInboundEmail.Handle(t.Context(), row.WorkspaceID, row.ID))
		assert.Len(t, conversations(tc), 1)

		ctx := tenancy.With(t.Context(), row.WorkspaceID.String())
		_, err := tc.App.DB.Exec(ctx, "UPDATE inbound_emails SET status = 'received', raw_path = 'missing.eml'")
		require.NoError(t, err)

		require.Error(t, tc.App.Actions.ProcessInboundEmail.Handle(t.Context(), row.WorkspaceID, row.ID))
		failed := lastInbound(tc, address)
		assert.Equal(t, "failed", failed.Status)
		assert.NotEmpty(t, *failed.Error)
	})

	t.Run("keeps a workspace's default address when repaired", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		before := defaultAddress(tc)

		_, err := tc.App.Actions.RepairWorkspaces.Handle(t.Context(), tc.Workspace.Workspace.Slug)
		require.NoError(t, err)

		assert.Equal(t, before.ID, defaultAddress(tc).ID)
		tc.Table("email_addresses").AssertCount(1)
	})

	t.Run("connects an address to the inbox the request names", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		ws := tc.Workspace.Workspace.ID
		inbox := query(tc, ws, func(ctx context.Context, q *models.Queries) (models.Inbox, error) {
			return q.CreateInbox(ctx, models.CreateInboxParams{
				WorkspaceID: ws, Name: "Billing", Settings: []byte(`{}`), Now: tc.Clock.Now(),
			})
		})

		tc.ActingAsOwner().PostJSON("/api/v1/email-addresses", map[string]any{
			"address": "billing@acme.test", "inbox_id": models.PublicID(models.PrefixInbox, inbox.ID),
		}).AssertCreated().AssertJSONPath("data.inbox.name", "Billing").AssertJSONPath("data.display_name",
			tc.Workspace.Workspace.Name)
	})
}
