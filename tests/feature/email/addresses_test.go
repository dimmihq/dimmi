package email_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestEmailAddresses(t *testing.T) {
	t.Parallel()

	t.Run("gives a new workspace a forwarding address named after its slug", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		res := tc.ActingAsOwner().GetJSON("/api/v1/email-addresses").AssertOK()

		slug := tc.Workspace.Workspace.Slug
		res.AssertJSONPath("data.#", 1.0)
		res.AssertJSONPath("data.0.object", "email_address")
		res.AssertJSONPath("data.0.forwarding_address", slug+"@"+inboundDomain)
		res.AssertJSONPath("data.0.address", slug+"@"+inboundDomain)
		res.AssertJSONPath("data.0.is_default", true)
		res.AssertJSONPath("data.0.inbox.name", "Support")
		res.AssertJSONPath("data.0.channel_connection.status", "active")
		res.AssertJSONPath("data.0.settings.quote_previous", true)
		res.AssertJSONPath("data.0.setup_guides.0.recommended", true)
		res.AssertJSONPath("data.0.sending_identity.uses_default_sender", true)
		res.AssertJSONPath("data.0.inbound_verified_at", nil)
		assert.Equal(t, slug+"+k7f3x9q2mz8b4c1d@"+inboundDomain, res.String("data.0.reply_to_preview"))
	})

	t.Run("connects a public address to a new forwarding address", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		owner := tc.ActingAsOwner()
		slug := tc.Workspace.Workspace.Slug

		res := owner.PostJSON("/api/v1/email-addresses", map[string]any{
			"address": "Sales@Acme.test", "display_name": "Acme Sales",
		}).AssertCreated()

		res.AssertJSONPath("data.address", "sales@acme.test")
		res.AssertJSONPath("data.forwarding_address", slug+".sales@"+inboundDomain)
		res.AssertJSONPath("data.display_name", "Acme Sales")
		res.AssertJSONPath("data.is_default", false)

		owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "sales@acme.test"}).
			AssertStatus(409).AssertJSONPath("code", "address_in_use")

		owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "sales@other.test"}).
			AssertCreated().AssertJSONPath("data.forwarding_address", slug+".sales-2@"+inboundDomain)

		owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "x@" + inboundDomain}).
			AssertStatus(422).AssertJSONPath("code", "invalid_address")
		owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": "not an address"}).
			AssertValidationError("address")
		owner.PostJSON("/api/v1/email-addresses", map[string]any{
			"address": "help@acme.test", "inbox_id": "inbox_00000000000000000000000000",
		}).AssertNotFound()
	})

	t.Run("changes how an address replies and handles mail", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)

		res := tc.ActingAsOwner().PatchJSON("/api/v1/email-addresses/"+publicID(address), map[string]any{
			"display_name": "Acme Help", "reply_to_mode": "none", "from_name_style": "name_only",
			"settings": map[string]any{"quote_previous": false, "bulk_mail_handling": "open"},
		}).AssertOK()

		res.AssertJSONPath("data.display_name", "Acme Help")
		res.AssertJSONPath("data.reply_to_mode", "none")
		res.AssertJSONPath("data.reply_to_preview", nil)
		res.AssertJSONPath("data.from_name_style", "name_only")
		res.AssertJSONPath("data.settings.quote_previous", false)
		res.AssertJSONPath("data.settings.bulk_mail_handling", "open")

		tc.ActingAsOwner().PatchJSON("/api/v1/email-addresses/"+publicID(address),
			map[string]any{"reply_to_mode": "never"}).AssertValidationError("reply_to_mode")
	})

	t.Run("sends an address's new conversations to another inbox", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		owner := tc.ActingAsOwner()
		it := owner.PostJSON("/api/v1/inboxes", map[string]any{"name": "IT"}).AssertCreated().String("data.id")

		owner.PatchJSON("/api/v1/email-addresses/"+publicID(address), map[string]any{"inbox_id": it}).
			AssertOK().AssertJSONPath("data.inbox.id", it)
		owner.PatchJSON("/api/v1/email-addresses/"+publicID(address), map[string]any{
			"inbox_id": "inbox_00000000000000000000000000",
		}).AssertValidationError("inbox_id")
	})

	t.Run("disconnects an address", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		owner := tc.ActingAsOwner()

		owner.Delete("/api/v1/email-addresses/" + publicID(address)).AssertNoContent()

		owner.GetJSON("/api/v1/email-addresses").AssertOK().AssertJSONPath("data.#", 0.0)
		owner.GetJSON("/api/v1/email-addresses/" + publicID(address)).AssertNotFound()
		tc.Table("channel_connections").AssertHas(map[string]any{"status": "disconnected"})
	})

	t.Run("lets members read addresses but only admins change them", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		user := tc.CreateUser()
		_, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)
		member := tc.ActingAs(user)

		member.GetJSON("/api/v1/email-addresses").AssertOK()
		member.GetJSON("/api/v1/email-addresses/" + publicID(address)).AssertOK()
		member.PostJSON("/api/v1/email-addresses", map[string]any{"address": "a@acme.test"}).AssertForbidden()
		member.PatchJSON("/api/v1/email-addresses/"+publicID(address), map[string]any{"display_name": "X"}).
			AssertForbidden()
		member.Delete("/api/v1/email-addresses/" + publicID(address)).AssertForbidden()
		member.PostJSON("/api/v1/email-addresses/"+publicID(address)+"/tests", nil).AssertForbidden()
		member.GetJSON("/api/v1/email-addresses/" + publicID(address) + "/inbound-emails").AssertForbidden()
	})

	t.Run("never shows another workspace's address", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		stranger := tc.CreateUser()
		_, err := factories.Workspace(t.Context(), tc.App, stranger)
		require.NoError(t, err)

		theirs := tc.ActingAs(stranger).GetJSON("/api/v1/email-addresses").AssertOK().String("data.0.id")
		require.NotEmpty(t, theirs)

		owner := tc.ActingAsOwner()
		owner.GetJSON("/api/v1/email-addresses/" + theirs).AssertNotFound()
		owner.PatchJSON("/api/v1/email-addresses/"+theirs, map[string]any{"display_name": "Mine"}).AssertNotFound()
		owner.Delete("/api/v1/email-addresses/" + theirs).AssertNotFound()
		owner.PostJSON("/api/v1/email-addresses/"+theirs+"/tests", nil).AssertNotFound()
		owner.GetJSON("/api/v1/email-addresses/" + theirs + "/inbound-emails").AssertNotFound()
		owner.GetJSON("/api/v1/email-addresses/ema_" + strings.Repeat("0", 26)).AssertNotFound()
		owner.GetJSON("/api/v1/email-addresses/not-an-id").AssertNotFound()
	})

	t.Run("proves forwarding works with a test email that comes back", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t, mailpitEnv(t))
		public := unique("support")
		owner := tc.ActingAsOwner()

		created := owner.PostJSON("/api/v1/email-addresses", map[string]any{"address": public}).AssertCreated()
		id := created.String("data.id")

		res := owner.PostJSON("/api/v1/email-addresses/"+id+"/tests", nil).AssertCreated()
		res.AssertJSONPath("data.object", "email_address_test")
		res.AssertJSONPath("data.sent_to", public)

		mails := received(t, public)
		require.Len(t, mails, 1)
		assert.NotEmpty(t, parsed(t, mails[0]).Header.Get("X-Dimmi-Test"))

		deliver(tc, mails[0], to(created.String("data.forwarding_address")))

		assert.NotEmpty(t, owner.GetJSON("/api/v1/email-addresses/"+id).AssertOK().String("data.inbound_verified_at"))
		assert.Empty(t, conversations(tc))

		activity := owner.GetJSON("/api/v1/email-addresses/" + id + "/inbound-emails").AssertOK()
		activity.AssertJSONPath("data.0.ignore_reason", "forwarding_test")
		activity.AssertJSONPath("data.0.classification", "system_notice")
	})

	t.Run("lists what arrived at an address a page at a time", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)
		address := defaultAddress(tc)
		owner := tc.ActingAsOwner()

		for _, id := range []string{"a", "b", "c"} {
			deliver(tc, fixture(t, "newsletter.eml", map[string]string{"TO": address.ForwardingAddress}),
				to(address.ForwardingAddress, "id", id))
		}

		base := "/api/v1/email-addresses/" + publicID(address) + "/inbound-emails"
		first := owner.GetJSON(base + "?limit=2").AssertOK()
		first.AssertJSONPath("data.#", 2.0)
		first.AssertJSONPath("data.0.object", "inbound_email")
		cursor := first.String("meta.next_cursor")
		require.NotEmpty(t, cursor)

		owner.GetJSON(base+"?limit=2&cursor="+cursor).AssertOK().AssertJSONPath("data.#", 1.0)
		owner.GetJSON(base+"?filter[status]=ignored").AssertOK().AssertJSONPath("data.#", 2.0)
		owner.GetJSON(base+"?filter[status]=bogus").AssertStatus(400).AssertJSONPath("code", "invalid_filter")
	})
}

func TestInboundSource(t *testing.T) {
	t.Parallel()

	t.Run("refuses a delivery without the source's secret", func(t *testing.T) {
		t.Parallel()
		tc, _ := emailCase(t)

		tc.Client().WithHeader("Authorization", "Bearer wrong").
			Send("POST", "/webhooks/test/inbound", []byte("Subject: x\r\n\r\nx")).AssertUnauthorized()
		tc.Client().WithHeader("Authorization", "Bearer "+testSecret).
			Send("POST", "/webhooks/test/inbound", []byte{}).AssertStatus(413)
	})
}
