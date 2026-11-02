package mime_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/mime"
)

func message() mail.Message {
	return mail.Message{
		From:    mail.Address{Name: "Shop", Email: "shop@example.com"},
		To:      []mail.Address{{Name: "Ana", Email: "ana@example.com"}},
		Bcc:     []mail.Address{{Email: "hidden@example.com"}},
		Subject: "Order shipped",
		Text:    "Your order is on its way.",
	}
}

func build(t *testing.T, msg mail.Message) string {
	t.Helper()

	builder := mime.NewBuilder(clock.NewFake(time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)))
	raw, err := builder.Build(msg)
	require.NoError(t, err)

	return string(raw)
}

func TestBuild(t *testing.T) {
	t.Parallel()

	t.Run("writes a text message dated by the clock", func(t *testing.T) {
		t.Parallel()

		raw := build(t, message())

		assert.Contains(t, raw, "Date: Mon, 02 Nov 2026 09:30:00 +0000\r\n")
		assert.Contains(t, raw, "Subject: Order shipped\r\n")
		assert.Contains(t, raw, `To: "Ana" <ana@example.com>`)
		assert.Contains(t, raw, "Content-Type: text/plain; charset=UTF-8")
		assert.Regexp(t, `Message-ID: <[a-z2-7]{26}@example\.com>`, raw)
	})

	t.Run("names neither the bcc recipients nor the software", func(t *testing.T) {
		t.Parallel()

		raw := build(t, message())

		assert.NotContains(t, raw, "hidden@example.com")
		assert.NotContains(t, raw, "User-Agent")
		assert.NotContains(t, raw, "X-Mailer")
	})

	t.Run("keeps a message id and headers the caller set", func(t *testing.T) {
		t.Parallel()
		msg := message()
		msg.Cc = []mail.Address{{Email: "bo@example.com"}}
		msg.ReplyTo = mail.Address{Email: "help@example.com"}
		msg.Headers = map[string]string{"message-id": "<own@example.com>", "in-reply-to": "<prev@example.com>"}

		raw := build(t, msg)

		assert.Contains(t, raw, "Message-ID: <own@example.com>\r\n")
		assert.Contains(t, raw, "In-Reply-To: <prev@example.com>\r\n")
		assert.Contains(t, raw, "Cc: <bo@example.com>\r\n")
		assert.Contains(t, raw, "Reply-To: <help@example.com>\r\n")
		assert.Equal(t, 1, strings.Count(raw, "Message-ID:"))
	})

	t.Run("writes alternative bodies with related and mixed parts", func(t *testing.T) {
		t.Parallel()
		msg := message()
		msg.HTML = `<p><img src="cid:logo@example.com"></p>`
		msg.Attachments = []mail.Attachment{
			{Filename: "a.pdf", ContentType: "application/pdf", Content: []byte("%PDF")},
			{Filename: "logo.png", Content: []byte("png"), Inline: true, ContentID: "logo@example.com"},
		}

		raw := build(t, msg)

		assert.Contains(t, raw, "multipart/mixed")
		assert.Contains(t, raw, "multipart/related")
		assert.Contains(t, raw, "multipart/alternative")
		assert.Contains(t, raw, "Content-Id: <logo@example.com>")
		assert.Contains(t, raw, "Content-Type: application/pdf")
	})

	t.Run("writes an html only message", func(t *testing.T) {
		t.Parallel()
		msg := message()
		msg.Text, msg.HTML = "", "<p>Hi</p>"

		raw := build(t, msg)

		assert.Contains(t, raw, "Content-Type: text/html; charset=UTF-8")
		assert.NotContains(t, raw, "multipart")
	})

	t.Run("omits an empty to header when only bcc is set", func(t *testing.T) {
		t.Parallel()
		msg := message()
		msg.To = nil

		raw := build(t, msg)

		assert.NotContains(t, raw, "\r\nTo:")
	})

	t.Run("returns a raw message as it is", func(t *testing.T) {
		t.Parallel()
		raw := "From: a@example.com\r\nTo: b@example.com\r\nSubject: x\r\n\r\nbody"

		assert.Equal(t, raw, build(t, mail.Message{Raw: []byte(raw)}))
	})

	t.Run("refuses an invalid message", func(t *testing.T) {
		t.Parallel()

		_, err := mime.NewBuilder(clock.System()).Build(mail.Message{Subject: "nobody"})

		require.ErrorIs(t, err, mail.ErrPermanent)
	})
}

func TestDecompose(t *testing.T) {
	t.Parallel()

	t.Run("returns a message without a document as it is", func(t *testing.T) {
		t.Parallel()

		got, err := mime.Decompose(message())

		require.NoError(t, err)
		assert.Equal(t, message(), got)
	})

	t.Run("reads back what the builder wrote", func(t *testing.T) {
		t.Parallel()
		msg := message()
		msg.Cc = []mail.Address{{Name: "Bo", Email: "bo@example.com"}}
		msg.ReplyTo = mail.Address{Name: "Help", Email: "help@example.com"}
		msg.HTML = "<p>On its way</p>"
		msg.Headers = map[string]string{"References": "<a@example.com> <b@example.com>"}
		msg.Attachments = []mail.Attachment{{Filename: "a.txt", ContentType: "text/plain", Content: []byte("note")}}
		raw := build(t, msg)

		got, err := mime.Decompose(mail.Message{Raw: []byte(raw), Tags: map[string]string{"kind": "x"}})
		require.NoError(t, err)

		assert.Nil(t, got.Raw)
		assert.Equal(t, msg.From, got.From)
		assert.Equal(t, msg.ReplyTo, got.ReplyTo)
		assert.Equal(t, msg.To, got.To)
		assert.Equal(t, msg.Cc, got.Cc)
		assert.Equal(t, msg.Subject, got.Subject)
		assert.Equal(t, msg.Text, strings.TrimSpace(got.Text))
		assert.Equal(t, msg.HTML, strings.TrimSpace(got.HTML))
		assert.Equal(t, msg.Headers["References"], got.Headers["References"])
		assert.NotContains(t, got.Headers, "Date")
		assert.Contains(t, got.Headers, "Message-Id")
		assert.Equal(t, msg.Attachments, got.Attachments)
		assert.Equal(t, "x", got.Tags["kind"])
	})

	t.Run("keeps addresses already set on the message", func(t *testing.T) {
		t.Parallel()
		raw := build(t, message())
		from := mail.Address{Email: "other@example.com"}
		bcc := []mail.Address{{Email: "z@example.com"}}

		got, err := mime.Decompose(mail.Message{Raw: []byte(raw), From: from, Bcc: bcc})
		require.NoError(t, err)

		assert.Equal(t, from, got.From)
		assert.Empty(t, got.To)
		assert.Equal(t, bcc, got.Bcc)
	})

	t.Run("reads an image without disposition as inline", func(t *testing.T) {
		t.Parallel()
		raw := "From: a@example.com\r\nTo: b@example.com\r\nContent-Type: multipart/related; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: text/html\r\n\r\n<img src=\"cid:i1\">\r\n" +
			"--b\r\nContent-Type: image/png\r\nContent-ID: <i1>\r\n\r\npng\r\n--b--\r\n"

		got, err := mime.Decompose(mail.Message{Raw: []byte(raw)})
		require.NoError(t, err)

		require.Len(t, got.Attachments, 1)
		assert.True(t, got.Attachments[0].Inline)
		assert.Equal(t, "i1", got.Attachments[0].ContentID)
	})

	t.Run("reads a document with unparseable addresses as having none", func(t *testing.T) {
		t.Parallel()

		got, err := mime.Decompose(mail.Message{Raw: []byte("From: <<x\r\nSubject: s\r\n\r\nbody")})

		require.NoError(t, err)
		assert.Empty(t, got.From.Email)
		assert.Equal(t, "s", got.Subject)
	})
}
