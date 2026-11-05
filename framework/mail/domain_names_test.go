package mail_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/mail"
)

func TestUnicodeDomain(t *testing.T) {
	t.Parallel()

	t.Run("shows an international domain in its own letters", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "café.example", mail.UnicodeDomain("xn--caf-dma.example"))
		assert.Equal(t, "acme.com", mail.UnicodeDomain("acme.com"))
	})

	t.Run("shows a name it cannot read as it is", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "xn--a.example", mail.UnicodeDomain("xn--a.example"))
	})
}
