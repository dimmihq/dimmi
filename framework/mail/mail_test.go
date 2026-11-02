package mail_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
)

func TestAddress(t *testing.T) {
	t.Parallel()

	t.Run("parses a name and an address", func(t *testing.T) {
		t.Parallel()

		got, err := mail.ParseAddress(`"Ana Lima" <ana@example.com>`)

		require.NoError(t, err)
		assert.Equal(t, mail.Address{Name: "Ana Lima", Email: "ana@example.com"}, got)
	})

	t.Run("refuses text that is no address", func(t *testing.T) {
		t.Parallel()

		_, err := mail.ParseAddress("ana at example")

		require.ErrorIs(t, err, mail.ErrInvalidAddress)
	})

	t.Run("formats names that need quoting or encoding", func(t *testing.T) {
		t.Parallel()

		quoted := mail.Address{Name: "Lima, Ana", Email: "ana@example.com"}
		encoded := mail.Address{Name: "José", Email: "jose@example.com"}

		assert.Equal(t, `"Lima, Ana" <ana@example.com>`, quoted.String())
		assert.Equal(t, "=?utf-8?q?Jos=C3=A9?= <jose@example.com>", encoded.String())
	})
}

func TestRateLimitedError(t *testing.T) {
	t.Parallel()

	t.Run("says when to retry when the provider did", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "mail: rate limited", (&mail.RateLimitedError{}).Error())
		limited := &mail.RateLimitedError{RetryAfter: 30 * time.Second}
		assert.Equal(t, "mail: rate limited, retry after 30s", limited.Error())
	})
}

func valid() mail.Message {
	return mail.Message{
		From:    mail.Address{Email: "sender@example.com"},
		To:      []mail.Address{{Email: "ana@example.com"}},
		Subject: "Hello",
		Text:    "Hi",
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	t.Run("accepts a complete message", func(t *testing.T) {
		t.Parallel()

		msg := valid()
		msg.ReplyTo = mail.Address{Name: "Help", Email: "help@example.com"}
		msg.Headers = map[string]string{"In-Reply-To": "<a@example.com>"}

		require.NoError(t, msg.Validate())
	})

	refusals := map[string]func(msg *mail.Message){
		"refuses a reserved header": func(msg *mail.Message) {
			msg.Headers = map[string]string{"bcc": "x@example.com"}
		},
		"refuses a header name with a colon": func(msg *mail.Message) {
			msg.Headers = map[string]string{"X-A: b": "c"}
		},
		"refuses an empty header name":         func(msg *mail.Message) { msg.Headers = map[string]string{"": "c"} },
		"refuses a line break in the subject":  func(msg *mail.Message) { msg.Subject = "a\nBcc: x@example.com" },
		"refuses a line break in a name":       func(msg *mail.Message) { msg.To[0].Name = "a\r\nb" },
		"refuses a reply-to without an email":  func(msg *mail.Message) { msg.ReplyTo = mail.Address{Name: "Help"} },
		"refuses an address with a name in it": func(msg *mail.Message) { msg.To[0].Email = "Ana <ana@example.com>" },
		"refuses a raw message that is no mime": func(msg *mail.Message) {
			*msg = mail.Message{Raw: []byte("no header block")}
		},
		"refuses a raw message with a broken from": func(msg *mail.Message) {
			*msg = mail.Message{Raw: []byte("From: <<broken\r\nTo: ana@example.com\r\n\r\nbody")}
		},
		"refuses a raw message with a broken to": func(msg *mail.Message) {
			*msg = mail.Message{Raw: []byte("From: a@example.com\r\nTo: <<broken\r\n\r\nbody")}
		},
	}

	for name, change := range refusals {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			msg := valid()
			change(&msg)

			require.ErrorIs(t, msg.Validate(), mail.ErrPermanent)
		})
	}
}

func TestRawAddresses(t *testing.T) {
	t.Parallel()

	raw := []byte("From: Ana <ana@example.com>\r\nTo: bo@example.com\r\nCc: Cy <cy@example.com>\r\n\r\nbody")

	t.Run("reads sender and recipients from the document", func(t *testing.T) {
		t.Parallel()
		msg := mail.Message{Raw: raw}

		from, err := msg.Sender()
		require.NoError(t, err)
		recipients, err := msg.Recipients()
		require.NoError(t, err)

		assert.Equal(t, mail.Address{Name: "Ana", Email: "ana@example.com"}, from)
		assert.Equal(t, []mail.Address{{Email: "bo@example.com"}, {Name: "Cy", Email: "cy@example.com"}}, recipients)
		require.NoError(t, msg.Validate())
	})

	t.Run("prefers the message fields over the document", func(t *testing.T) {
		t.Parallel()
		msg := mail.Message{
			Raw: raw, From: mail.Address{Email: "x@example.com"}, Bcc: []mail.Address{{Email: "z@example.com"}},
		}

		from, err := msg.Sender()
		require.NoError(t, err)
		recipients, err := msg.Recipients()
		require.NoError(t, err)

		assert.Equal(t, "x@example.com", from.Email)
		assert.Equal(t, []mail.Address{{Email: "z@example.com"}}, recipients)
	})

	t.Run("finds no sender in a document without one", func(t *testing.T) {
		t.Parallel()
		msg := mail.Message{Raw: []byte("To: bo@example.com\r\n\r\nbody")}

		from, err := msg.Sender()

		require.NoError(t, err)
		assert.Empty(t, from.Email)
		require.ErrorIs(t, msg.Validate(), mail.ErrPermanent)
	})

	t.Run("finds no recipients in a document without them", func(t *testing.T) {
		t.Parallel()
		msg := mail.Message{Raw: []byte("From: a@example.com\r\n\r\nbody")}

		require.ErrorIs(t, msg.Validate(), mail.ErrPermanent)
	})
}
