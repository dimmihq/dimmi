package email_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/integrations/email"
)

func TestAddressing(t *testing.T) {
	t.Parallel()

	sample := email.NewReplyToken()

	t.Run("reads forwarding addresses and reply tokens on the inbound domain only", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			address string
			want    email.Recipient
			ok      bool
		}{
			{"Acme@Inbound.test", email.Recipient{Forwarding: "acme@inbound.test"}, true},
			{
				"acme+" + sample + "@inbound.test",
				email.Recipient{Forwarding: "acme@inbound.test", Token: sample},
				true,
			},
			{"acme+unsub-xyz@inbound.test", email.Recipient{Forwarding: "acme@inbound.test"}, true},
			{"acme@elsewhere.test", email.Recipient{}, false},
			{"+token@inbound.test", email.Recipient{}, false},
			{"no-at-sign", email.Recipient{}, false},
		}

		for _, c := range cases {
			got, ok := email.ParseRecipient(c.address, "inbound.test")
			assert.Equal(t, c.ok, ok, c.address)
			assert.Equal(t, c.want, got, c.address)
		}

		_, ok := email.ParseRecipient("acme@inbound.test", "")
		assert.False(t, ok)
	})

	t.Run("makes tokens of the right shape and puts them in the local part", func(t *testing.T) {
		t.Parallel()

		token := email.NewReplyToken()
		assert.True(t, email.ValidToken(token))
		assert.NotEqual(t, token, email.NewReplyToken())
		assert.False(t, email.ValidToken("short"))
		assert.False(t, email.ValidToken("k7f3x9q2mz8b4c1u"))
		assert.Equal(t, "acme+"+token+"@inbound.test", email.ReplyAddress("acme@inbound.test", token))
	})

	t.Run("names forwarding addresses after the slug, never a reserved name", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "acme", email.DefaultLocal("acme"))
		assert.Equal(t, "support-team", email.DefaultLocal("support"))
		assert.True(t, email.Reserved("postmaster"))
		assert.False(t, email.Reserved("acme"))
		assert.Equal(t, "acme.sales", email.LabeledLocal("acme", "Sales@acme.test"))
		assert.Equal(t, "acme.first-last", email.LabeledLocal("acme", "first.last@acme.test"))
		assert.Equal(t, "acme.mail", email.LabeledLocal("acme", "@acme.test"))
		assert.Equal(t, "acme."+strings.Repeat("a", 30), email.LabeledLocal("acme", strings.Repeat("a", 40)+"@x.test"))
		assert.Equal(t, "acme", email.WithSuffix("acme", 1))
		assert.Equal(t, "acme-3", email.WithSuffix("acme", 3))
	})
}

func TestSubjects(t *testing.T) {
	t.Parallel()

	t.Run("strips reply prefixes in many languages and gateway tags", func(t *testing.T) {
		t.Parallel()

		for subject, want := range map[string]string{
			"Re: Refund":               "refund",
			"RE: Fwd: AW: Refund":      "refund",
			"[EXTERNAL] Re[2]: Refund": "refund",
			"回复：退款":                    "退款",
			"Sv:   Order   #42 ":       "order #42",
			"Re: Refund [#1042]":       "refund",
			"Refund for order":         "refund for order",
		} {
			assert.Equal(t, want, email.NormalizeSubject(subject), subject)
		}

		assert.True(t, email.HasReplyPrefix("[External] RE: Hello"))
		assert.False(t, email.HasReplyPrefix("Hello"))
		assert.Equal(t, "Refund For Order", email.DisplaySubject("Re: Refund  For Order"))
		assert.Len(t, []rune(email.DisplaySubject(strings.Repeat("é", 300))), 255)
	})

	t.Run("trusts only specific subjects", func(t *testing.T) {
		t.Parallel()

		assert.True(t, email.SpecificSubject("refund for order 1042"))
		assert.False(t, email.SpecificSubject("help"))
		assert.False(t, email.SpecificSubject("question"))
		assert.False(t, email.SpecificSubject("quick question"))
	})
}
