package emaildomains_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

func TestReplyStyle(t *testing.T) {
	t.Parallel()

	t.Run("frames replies in the brand's color and footer once an address turns branded", func(t *testing.T) {
		t.Parallel()
		s := newSignatureCase(t)
		author := s.memberAuthor(s.member, "Sam")

		plain := signedReply(s.tc, s.conv, author, "")
		assert.NotContains(t, plain.HTML, "<table")

		s.owner.PatchJSON("/api/v1/email-addresses/"+s.address, map[string]any{"settings": map[string]any{
			"template": "branded", "brand_color": "#FF5500", "footer_markdown": "**Acme** · [Help](https://acme.test/help)",
		}}).AssertOK().
			AssertJSONPath("data.settings.template", "branded").
			AssertJSONPath("data.settings.brand_color", "#ff5500")

		reply := signedReply(s.tc, s.conv, author, "")
		assert.Contains(t, reply.HTML, `<table role="presentation"`)
		assert.Contains(t, reply.HTML, "border-top:2px solid #ff5500")
		assert.Contains(t, reply.HTML, `<strong>Acme</strong> · <a href="https://acme.test/help"`)
		assert.Contains(t, reply.HTML, "The refund is on its way.")
		assert.NotContains(t, reply.HTML, "<img")
		assert.Contains(t, reply.Text, "The refund is on its way.")
		assert.Contains(t, reply.Text, "**Acme** · [Help](https://acme.test/help)")

		agent := contracts.OutboundAuthor{Type: enums.AuthorTypeAgent, ID: ids.New(), Name: "Assistant"}
		assert.Contains(t, signedReply(s.tc, s.conv, agent, "").HTML, "border-top:2px solid #ff5500")

		s.owner.PatchJSON("/api/v1/email-addresses/"+s.address, map[string]any{"settings": map[string]any{
			"template": "plain",
		}}).AssertOK()
		assert.NotContains(t, signedReply(s.tc, s.conv, author, "").HTML, "<table")
	})

	t.Run("previews a style before it is saved and refuses what is not one", func(t *testing.T) {
		t.Parallel()
		s := newSignatureCase(t)
		url := "/api/v1/email-addresses/" + s.address + "/template-renderings"

		shown := s.owner.PostJSON(url, map[string]any{
			"template": "branded", "brand_color": "#00aa88", "footer_markdown": "Acme Inc. <script>x</script>",
		}).AssertCreated()
		shown.AssertJSONPath("data.object", "email_template_rendering")
		assert.Contains(t, shown.String("data.html"), "border-top:2px solid #00aa88")
		assert.Contains(t, shown.String("data.html"), "Acme Inc.")
		assert.NotContains(t, shown.String("data.html"), "<script")
		assert.NotContains(t, shown.String("data.html"), `src="http`)
		assert.Contains(t, shown.String("data.text"), "Hi Alex,")

		bare := s.owner.PostJSON(url, map[string]any{"template": "branded"}).AssertCreated()
		assert.Contains(t, bare.String("data.html"), "border-top:2px solid #171717")
		assert.NotContains(t, bare.String("data.html"), "border-top:1px solid #e5e5e5;color:#737373;font-size:13px")

		saved := s.owner.PostJSON(url, map[string]any{}).AssertCreated()
		assert.NotContains(t, saved.String("data.html"), "<table")

		s.owner.PostJSON(url, map[string]any{"brand_color": "red"}).AssertStatus(422)
		s.owner.PatchJSON("/api/v1/email-addresses/"+s.address, map[string]any{"settings": map[string]any{
			"template": "fancy",
		}}).AssertStatus(422)
		s.asMember.PostJSON(url, map[string]any{"template": "branded"}).AssertStatus(403)
		s.owner.PostJSON("/api/v1/email-addresses/ema_missing/template-renderings", map[string]any{}).
			AssertStatus(404)
	})

	t.Run("moves the default to another address and never turns it off", func(t *testing.T) {
		t.Parallel()
		s := newSignatureCase(t)

		other := s.owner.PostJSON("/api/v1/email-addresses", map[string]any{
			"address": "billing@acme.test", "display_name": "Acme Billing",
		}).AssertCreated().String("data.id")

		list := s.owner.GetJSON("/api/v1/email-addresses").AssertOK()
		current := list.String("data.0.id")
		assert.NotEqual(t, other, current)

		s.owner.PatchJSON("/api/v1/email-addresses/"+current, map[string]any{"is_default": false}).
			AssertStatus(422).AssertJSONPath("code", "default_address_required")
		s.asMember.PatchJSON("/api/v1/email-addresses/"+other, map[string]any{"is_default": true}).
			AssertStatus(403)

		s.owner.PatchJSON("/api/v1/email-addresses/"+other, map[string]any{"is_default": true}).
			AssertOK().AssertJSONPath("data.is_default", true)
		s.owner.GetJSON("/api/v1/email-addresses/"+current).AssertOK().AssertJSONPath("data.is_default", false)
		s.owner.GetJSON("/api/v1/email-addresses").AssertJSONPath("data.0.id", other)
	})
}
