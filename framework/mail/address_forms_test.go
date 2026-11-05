package mail_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
)

func TestNormalizeAddress(t *testing.T) {
	t.Parallel()

	same := []struct{ name, a, b string }{
		{"composed and decomposed accents", "josé@example.test", "josé@example.test"},
		{"fullwidth letters", "ａnn@example.test", "ann@example.test"},
		{"the long s", "ſam@example.test", "sam@example.test"},
		{"a unicode domain and its ascii form", "a@bücher.de", "a@xn--bcher-kva.de"},
		{"upper and lower case", " Ann@Example.TEST ", "ann@example.test"},
	}

	for _, c := range same {
		t.Run("treats "+c.name+" as one address", func(t *testing.T) {
			t.Parallel()

			a, err := mail.NormalizeAddress(c.a)
			require.NoError(t, err)
			b, err := mail.NormalizeAddress(c.b)
			require.NoError(t, err)
			assert.Equal(t, b, a)
		})
	}

	for name, raw := range map[string]string{
		"a zero-width space": "x\u200b@example.test", "a control character": "x\u0007@example.test",
		"no local part": "@example.test", "no domain": "ann@", "no at sign": "ann.example.test",
		"a broken domain": "ann@exa mple.test",
	} {
		t.Run("refuses an address with "+name, func(t *testing.T) {
			t.Parallel()

			_, err := mail.NormalizeAddress(raw)
			require.ErrorIs(t, err, mail.ErrInvalidAddress)
		})
	}

	t.Run("falls back to the trimmed lowercase address for lookups", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "a@xn--bcher-kva.de", mail.CanonicalAddress(" A@B\u00fccher.de "))
		assert.Equal(t, "x\u200b@example.test", mail.CanonicalAddress("X\u200b@Example.test"))
	})
}
