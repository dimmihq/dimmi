package parse_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mailbox/parse"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := fs.ReadFile(os.DirFS("testdata"), name)
	require.NoError(t, err)

	return raw
}

func parsed(t *testing.T, raw []byte, limits parse.Limits) *parse.Parsed {
	t.Helper()

	got, err := parse.Message(bytes.NewReader(raw), limits)
	require.NoError(t, err)

	return got
}

func TestMessage(t *testing.T) {
	t.Parallel()

	t.Run("reads addresses, threading headers and both bodies", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, fixture(t, "alternative.eml"), parse.Limits{})

		assert.Equal(t, mail.Address{Name: "Ana Lima", Email: "ana@sender.example"}, got.From)
		to := []mail.Address{{Name: "Support", Email: "help@shop.example"}, {Email: "bo@shop.example"}}
		assert.Equal(t, to, got.To)
		assert.Equal(t, []mail.Address{{Name: "José", Email: "jose@sender.example"}}, got.Cc)
		assert.Equal(t, []mail.Address{{Name: "Ana Replies", Email: "replies@sender.example"}}, got.ReplyTo)
		assert.Equal(t, "Re: Order résumé", got.Subject)
		assert.Equal(t, time.Date(2026, 11, 2, 9, 4, 0, 0, time.UTC), got.Date.UTC())
		assert.Equal(t, "m3.abc@sender.example", got.MessageID)
		assert.Equal(t, []string{"m2.def@shop.example"}, got.InReplyTo)
		assert.Equal(t, []string{"m1@sender.example", "m2.def@shop.example"}, got.References)
		assert.Equal(t, "Thanks, the résumé arrived.", strings.TrimSpace(got.Text))
		assert.Equal(t, "<p>Thanks, the résumé arrived.</p>", strings.TrimSpace(got.HTML))
		assert.Equal(t, "<ana@sender.example>", got.Header.Get("Return-Path"))
		assert.False(t, got.Truncated)
	})

	t.Run("keeps attachments and inline images", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, fixture(t, "attachments.eml"), parse.Limits{})

		require.Len(t, got.Attachments, 2)
		assert.Equal(t, parse.Attachment{
			Filename: "invoice.pdf", ContentType: "application/pdf", Size: 16, Content: []byte("%PDF-1.4 invoice"),
		}, got.Attachments[0])
		assert.Equal(t, "logo.png", got.Attachments[1].Filename)
		assert.Equal(t, "logo@sender.example", got.Attachments[1].ContentID)
		assert.True(t, got.Attachments[1].Inline)
	})

	t.Run("decodes a declared charset and cleans the text", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, fixture(t, "latin1.eml"), parse.Limits{})

		assert.Equal(t, "Café", got.Subject)
		assert.Equal(t, "Un café avec sucre.", strings.TrimSpace(got.Text))
		assert.Equal(t, "moc.knab", got.From.Name)
		assert.True(t, got.Date.IsZero())
		assert.Contains(t, got.Warnings, "unreadable date: not a date")
	})

	t.Run("keeps what it read from a broken structure as warnings", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, fixture(t, "unclosed.eml"), parse.Limits{})

		assert.Equal(t, "Body before the end.", strings.TrimSpace(got.Text))
		assert.NotEmpty(t, got.Warnings)
	})

	t.Run("reads a message with too many parts as plain text", func(t *testing.T) {
		t.Parallel()
		raw := fixture(t, "attachments.eml")

		got := parsed(t, raw, parse.Limits{MaxParts: 1})

		assert.Equal(t, string(raw), got.Text)
		assert.Len(t, got.Warnings, 1)
	})

	t.Run("caps the warnings it keeps", func(t *testing.T) {
		t.Parallel()
		var raw strings.Builder
		raw.WriteString("From: a@example.com\r\nDate: never\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n")
		for range 5 {
			raw.WriteString("--b\r\nContent-Type: text/plain; charset=bogus\r\n\r\nx\r\n")
		}

		got := parsed(t, []byte(raw.String()), parse.Limits{MaxWarnings: 2})

		assert.Len(t, got.Warnings, 2)
	})

	t.Run("reports a source that fails", func(t *testing.T) {
		t.Parallel()
		errRead := errors.New("disk gone")

		_, err := parse.Message(iotest.ErrReader(errRead), parse.Limits{})

		require.ErrorIs(t, err, errRead)
	})
}

/*
large returns a message whose attachment pushes it past a small size
limit.
*/
func large(textCharset, text string) string {
	return "From: ana@sender.example\r\nTo: help@shop.example\r\nSubject: Big\r\n" +
		"Message-ID: <big@sender.example>\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n" +
		"--b\r\nContent-Type: text/plain; charset=" + textCharset + "\r\n\r\n" + text + "\r\n" +
		"--b\r\nContent-Type: text/html\r\n\r\n<p>" + strings.Repeat("h", 100) + "</p>\r\n" +
		"--b\r\nContent-Type: image/png\r\nContent-Disposition: inline\r\nContent-ID: <i1>\r\n\r\npng\r\n" +
		"--b\r\nContent-Type: application/zip\r\nContent-Disposition: attachment; filename=\"big.zip\"\r\n\r\n" +
		strings.Repeat("z", 2000) + "\r\n--b--\r\n"
}

func TestOversized(t *testing.T) {
	t.Parallel()

	limits := parse.Limits{MaxBytes: 1000, MaxPartBytes: 20}

	t.Run("keeps headers and capped bodies and lists attachments as dropped", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte(large("utf-8", "Hello there, this is the visible text.")), limits)

		assert.True(t, got.Truncated)
		assert.Equal(t, "Big", got.Subject)
		assert.Equal(t, "big@sender.example", got.MessageID)
		assert.Equal(t, "Hello there, this is", got.Text)
		assert.Equal(t, "<p>hhhhhhhhhhhhhhhhh", got.HTML)
		require.Len(t, got.Attachments, 2)
		assert.Equal(t, parse.Attachment{
			ContentType: "image/png", ContentID: "i1", Inline: true, Size: 3, Dropped: true,
		}, got.Attachments[0])
		assert.Equal(t, parse.Attachment{
			Filename: "big.zip", ContentType: "application/zip", Size: 2000, Dropped: true,
		}, got.Attachments[1])
	})

	t.Run("decodes a charset the stream reader does not know", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte(large("iso-8859-1", "caf\xe9")), limits)

		assert.Equal(t, "café", got.Text)
	})

	t.Run("reads the bytes of an unknown charset as well as it can", func(t *testing.T) {
		t.Parallel()

		got := parsed(t, []byte(large("x-nonsense", "caf\xe9")), limits)

		assert.Equal(t, "café", got.Text)
	})

	t.Run("joins the text parts of a large message within the cap", func(t *testing.T) {
		t.Parallel()

		raw := "From: ana@sender.example\r\nContent-Type: multipart/mixed; boundary=b\r\n\r\n" +
			"--b\r\nContent-Type: text/plain\r\n\r\nFirst\r\n" +
			"--b\r\nContent-Type: application/zip\r\nContent-Disposition: attachment; filename=\"big.zip\"\r\n\r\n" +
			strings.Repeat("z", 2000) + "\r\n" +
			"--b\r\nContent-Type: text/plain\r\n\r\nSecond\r\n--b--\r\n"

		got := parsed(t, []byte(raw), limits)

		assert.Equal(t, "First\n\nSecond", got.Text)
	})

	t.Run("warns about a structure it cannot read", func(t *testing.T) {
		t.Parallel()
		raw := "Subject: Big\r\nbroken header line\r\n\r\n" + strings.Repeat("x", 2000)

		got := parsed(t, []byte(raw), limits)

		assert.True(t, got.Truncated)
		assert.NotEmpty(t, got.Warnings)
	})

	t.Run("reports a source that fails while walking", func(t *testing.T) {
		t.Parallel()
		errRead := errors.New("connection reset")
		cut := large("utf-8", "text")[:1500]
		raw := io.MultiReader(strings.NewReader(cut), iotest.ErrReader(errRead))

		_, err := parse.Message(raw, parse.Limits{MaxBytes: 1000})

		require.ErrorIs(t, err, errRead)
	})
}
