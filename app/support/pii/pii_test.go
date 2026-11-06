package pii_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/pii"
)

func TestRedact(t *testing.T) {
	t.Parallel()

	r := pii.New()

	cases := []struct {
		name, in, want string
	}{
		{"masks an email", "write to jane.doe@corp.example now", "write to [email] now"},
		{"masks a luhn-valid card showing its last four", "card 4242 4242 4242 4242 ok", "card [card ending 4242] ok"},
		{"keeps a long order number that fails luhn", "order 1234567890123 shipped", "order 1234567890123 shipped"},
		{"keeps a short order number", "order #A-55821 is late", "order #A-55821 is late"},
		{"keeps a date", "since 2026-01-15 it fails", "since 2026-01-15 it fails"},
		{"keeps an invoice id", "invoice INV-2026-00042", "invoice INV-2026-00042"},
		{"masks an international phone", "call +49 30 1234 5678 please", "call [phone] please"},
		{"masks a us phone", "call (415) 555-2671 today", "call [phone] today"},
		{"masks an iban", "IBAN DE89 3704 0044 0532 0130 00 thanks", "IBAN [iban] thanks"},
		{"masks an ssn", "ssn 123-45-6789", "ssn [ssn]"},
		{"masks an openai key", "key sk-proj-abcdefghijklmnop1234", "key [secret]"},
		{"masks a github token", "ghp_abcdefghijklmnopqrstuvwxyz0123", "[secret]"},
		{"masks an aws key", "AKIAIOSFODNN7EXAMPLE", "[secret]"},
		{"masks a bearer token keeping the label", "Bearer abcdef123456789xyz", "Bearer [secret]"},
		{"masks a password keeping the label", "password: hunter2hunter2", "password: [secret]"},
		{
			"masks a jwt", "token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N",
			"token [secret]",
		},
		{"leaves empty text alone", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, r.Redact(c.in))
		})
	}
}
