package channels_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/contracts"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
)

func TestWhatsAppFormat(t *testing.T) {
	t.Parallel()

	t.Run("turns Markdown emphasis into WhatsApp's markup", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "*bold* and _italic_ and ~gone~", wadriver.ToWhatsApp("**bold** and *italic* and ~~gone~~"))
		assert.Equal(t, "*bold* _italic_", wadriver.ToWhatsApp("__bold__ _italic_"))
	})

	t.Run("makes headings bold and shows links with their address", func(t *testing.T) {
		t.Parallel()
		out := wadriver.ToWhatsApp("## Next steps\nRead [the guide](https://a.test/g) or https://b.test\n![chart](https://c.test/p.png)")
		assert.Equal(t, "*Next steps*\nRead the guide (https://a.test/g) or https://b.test\nhttps://c.test/p.png", out)
		assert.Equal(t, "https://a.test", wadriver.ToWhatsApp("[https://a.test](https://a.test)"))
	})

	t.Run("sends escaped characters as themselves", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "Thanks to **Hivy Support**. 1. ok",
			wadriver.ToWhatsApp(`Thanks to \*\*Hivy Support\*\*. 1\. ok`))
		assert.Equal(t, "price_list v2.0 (beta)", wadriver.ToWhatsApp(`price\_list v2\.0 \(beta\)`))
	})

	t.Run("leaves code alone", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "Run `**not bold**` now", wadriver.ToWhatsApp("Run `**not bold**` now"))
		assert.Equal(t, "```\n**raw**\n```", wadriver.ToWhatsApp("```go\n**raw**\n```"))
	})

	t.Run("keeps list bullets as dashes", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "- one\n- two\n1. three", wadriver.ToWhatsApp("* one\n- two\n1. three"))
	})

	t.Run("reads a customer's markup as Markdown", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "**urgent** *please* ~~old~~", wadriver.ToMarkdown("*urgent* _please_ ~old~"))
		assert.Equal(t, "2*3*4 = 24", wadriver.ToMarkdown("2*3*4 = 24"))
		assert.Equal(t, "snake_case_name", wadriver.ToMarkdown("snake_case_name"))
	})

	t.Run("splits long text near the limit at a line end", func(t *testing.T) {
		t.Parallel()
		parts := wadriver.Split("aaaa bbbb\ncccc dddd", 12)
		assert.Equal(t, []string{"aaaa bbbb", "cccc dddd"}, parts)
		assert.Equal(t, []string{"short"}, wadriver.Split("short", 10))
	})
}

func TestWhatsAppSignature(t *testing.T) {
	t.Parallel()

	body := []byte(`{"object":"whatsapp_business_account"}`)
	header := wadriver.Sign("secret", body)

	assert.True(t, wadriver.Verify("secret", header, body))
	assert.False(t, wadriver.Verify("other", header, body))
	assert.False(t, wadriver.Verify("secret", header, append(body, ' ')))
	assert.False(t, wadriver.Verify("", header, body))
	assert.False(t, wadriver.Verify("secret", "", body))
}

func TestWhatsAppKeys(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "whatsapp:u:US.13491208655302741918", wadriver.ThreadKey("US.13491208655302741918", "15551234567"))
	assert.Equal(t, "whatsapp:p:15551234567", wadriver.ThreadKey("", "15551234567"))
	assert.Empty(t, wadriver.ThreadKey("", ""))

	to, ok := wadriver.ParseThreadKey("whatsapp:p:15551234567")
	require.True(t, ok)
	assert.Equal(t, "15551234567", to.WaID)

	to, ok = wadriver.ParseThreadKey("whatsapp:u:US.1")
	require.True(t, ok)
	assert.Equal(t, "US.1", to.UserID)

	_, ok = wadriver.ParseThreadKey("slack:d:D1")
	assert.False(t, ok)
	assert.Equal(t, "+15551234567", wadriver.PhoneOf("15551234567"))
}

func TestWhatsAppDelivery(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"object":"whatsapp_business_account","entry":[{"id":"WABA1","changes":[{"field":"messages","value":{
		"messaging_product":"whatsapp","metadata":{"display_phone_number":"15551564184","phone_number_id":"PN1"},
		"contacts":[{"profile":{"name":"Ada"},"wa_id":"447700900123","user_id":"GB.42"}],
		"messages":[{"from":"447700900123","from_user_id":"GB.42","id":"wamid.A",
			"timestamp":"1795161600","type":"image",
			"image":{"id":"M1","mime_type":"image/jpeg","caption":"the bug"},
			"context":{"from":"15551564184","id":"wamid.Z"}}],
		"statuses":[{"id":"wamid.B","status":"read","timestamp":"1795161700","recipient_id":"447700900123",
			"pricing":{"billable":true,"pricing_model":"PMP","category":"service","type":"regular"}}]}}]}]}`)

	d, err := wadriver.ParseDelivery(raw)
	require.NoError(t, err)
	require.Len(t, d.Entry, 1)
	assert.Equal(t, "WABA1", d.Entry[0].ID)

	m, err := wadriver.ParseMessages(d.Entry[0].Changes[0].Value)
	require.NoError(t, err)
	assert.Equal(t, "PN1", m.Metadata.PhoneNumberID)
	assert.Equal(t, "GB.42", m.Contacts[0].UserID)
	require.Len(t, m.Messages, 1)

	msg := m.Messages[0]
	assert.Equal(t, "the bug", msg.Media().Caption)
	assert.Equal(t, "wamid.Z", msg.Context.ID)
	assert.Equal(t, time.Unix(1795161600, 0).UTC(), msg.SentAt(time.Time{}))
	assert.Equal(t, "read", m.Statuses[0].Status)
	assert.Equal(t, "service", m.Statuses[0].Pricing.Category)
}

func TestWhatsAppTemplates(t *testing.T) {
	t.Parallel()

	tpl := wadriver.Template{
		Name: "order_update", Language: "en_US", Status: "APPROVED",
		Components: []wadriver.Component{
			{Type: "HEADER", Format: "TEXT", Text: "Order {{1}}"},
			{Type: "BODY", Text: "Hi {{1}}, your order {{2}} ships {{3}}. Thanks {{1}}!"},
			{Type: "FOOTER", Text: "dimmi"},
			{Type: "BUTTONS", Buttons: []wadriver.Action{{Type: "QUICK_REPLY", Text: "Track"}}},
		},
	}

	assert.True(t, tpl.Sendable())
	assert.Equal(t, []wadriver.Variable{
		{Part: "header", Key: "1"}, {Part: "body", Key: "1"}, {Part: "body", Key: "2"}, {Part: "body", Key: "3"},
	}, tpl.Variables())

	out := tpl.Render(map[string]string{"header.1": "#42", "body.1": "Ada", "body.2": "#42"})
	assert.Equal(t, "**Order #42**\n\nHi Ada, your order #42 ships {{3}}. Thanks Ada!\n\n_dimmi_\n\n[Track]", out)

	media := tpl
	media.Components = []wadriver.Component{{Type: "HEADER", Format: "IMAGE"}, {Type: "BODY", Text: "Hi"}}
	assert.False(t, media.Sendable())

	pending := tpl
	pending.Status = "PENDING"
	assert.False(t, pending.Sendable())

	opts, err := wadriver.ParseOptions([]byte(`{"whatsapp":{"template":{"name":"order_update",` +
		`"language":"en_US","values":{"body.1":"Ada"}}}}`))
	require.NoError(t, err)
	require.NotNil(t, opts.Template)
	assert.Equal(t, "Ada", opts.Template.Values["body.1"])
}

func TestWhatsAppErrors(t *testing.T) {
	t.Parallel()

	cases := map[int]error{
		131047: contracts.ErrWindowClosed,
		131026: contracts.ErrRecipientUnreachable,
		132001: contracts.ErrContentRejected,
		190:    contracts.ErrConnectionBroken,
		131042: contracts.ErrPermanent,
	}

	for code, want := range cases {
		err := wadriver.MapError(&wadriver.GraphError{Code: code})
		assert.ErrorIs(t, err, want, "code %d", code)
	}

	limited, ok := errors.AsType[*contracts.RateLimitedError](wadriver.MapError(&wadriver.GraphError{Code: 131056}))
	require.True(t, ok)
	assert.Equal(t, 6*time.Second, limited.RetryAfter)

	transient := wadriver.MapError(&wadriver.GraphError{Code: 131000})
	assert.NotErrorIs(t, transient, contracts.ErrPermanent)

	plain := errors.New("timeout")
	assert.Same(t, plain, wadriver.MapError(plain))
}

func TestWhatsAppTemplateRefusals(t *testing.T) {
	t.Parallel()

	choice := wadriver.Choice{Name: "order_update", Language: "en_US"}
	body := []wadriver.Component{{Type: "BODY", Text: "Hi {{1}}, order {{2}} shipped."}}
	approved := wadriver.Template{Name: "order_update", Status: "APPROVED", Components: body}

	t.Run("names a template Meta does not have in the language", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "Meta has no template order_update in en_US. Pick another with /.",
			wadriver.RefusalReason(132001, choice, &approved))
		assert.Contains(t, wadriver.RefusalReason(0, choice, nil), "Meta has no template")
	})

	t.Run("says what Meta says of the template now before its code", func(t *testing.T) {
		t.Parallel()
		paused := approved
		paused.Status = "PAUSED"
		assert.Contains(t, wadriver.RefusalReason(132000, choice, &paused), "Meta paused order_update")

		rejected := approved
		rejected.Status, rejected.RejectedReason = "REJECTED", "INVALID_FORMAT"
		assert.Equal(t, "Meta rejected order_update (invalid format).",
			wadriver.RefusalReason(0, choice, &rejected))

		pending := approved
		pending.Status = "PENDING"
		assert.Equal(t, "order_update is still waiting for Meta's review.",
			wadriver.RefusalReason(0, choice, &pending))
	})

	t.Run("explains an approved template's refusal by Meta's code", func(t *testing.T) {
		t.Parallel()
		assert.Contains(t, wadriver.RefusalReason(132000, choice, &approved), "it now takes 2 variables")
		assert.Contains(t, wadriver.RefusalReason(132005, choice, &approved), "longer than WhatsApp allows")
		assert.Contains(t, wadriver.RefusalReason(132012, choice, &approved), "not in the form")
		assert.Equal(t, "Meta refused order_update (error 132999).",
			wadriver.RefusalReason(132999, choice, &approved))
	})

	t.Run("says when dimmi cannot send an approved template's kind", func(t *testing.T) {
		t.Parallel()
		media := approved
		media.Components = append([]wadriver.Component{{Type: "HEADER", Format: "IMAGE"}}, body...)
		assert.Contains(t, wadriver.RefusalReason(0, choice, &media), "dimmi cannot send order_update yet")
	})
}
