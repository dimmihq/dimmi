/*
|--------------------------------------------------------------------------
| Mailer Contract
|--------------------------------------------------------------------------
|
| Every mailer must deliver the same message the same way, or switching
| provider would change what recipients see. A driver's tests call Run
| with a constructor returning a mailer and a way to read back what its
| transport received for one recipient, from a real server or a fake at
| the provider boundary. The same suite runs against every driver and
| against the mail fake.
|
|   func TestMailer(t *testing.T) {
|       drivertest.Run(t, newHarness)
|   }
|
*/

package drivertest

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/mime"
)

/*
Harness is a mailer under test. Received returns the message the
transport received for the recipient address, with its fields read
back from whatever the transport got, and fails the test when none
arrived.
*/
type Harness struct {
	Mailer   mail.Mailer
	Received func(t *testing.T, recipient string) mail.Message
}

/*
NewHarness returns a mailer no other test uses.
*/
type NewHarness func(t *testing.T) Harness

/*
Run runs the contract against mailers from the constructor.
*/
func Run(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("content", func(t *testing.T) { t.Parallel(); runContent(t, newHarness) })
	t.Run("attachments", func(t *testing.T) { t.Parallel(); runAttachments(t, newHarness) })
	t.Run("recipients", func(t *testing.T) { t.Parallel(); runRecipients(t, newHarness) })
	t.Run("raw", func(t *testing.T) { t.Parallel(); runRaw(t, newHarness) })
	t.Run("refusals", func(t *testing.T) { t.Parallel(); runRefusals(t, newHarness) })
}

/*
Address returns a recipient address no other test uses.
*/
func Address(name string) mail.Address {
	return mail.Address{Name: name, Email: strings.ToLower(name+"-"+rand.Text()[:10]) + "@example.com"}
}

/*
Message returns a message from a sender to the recipient with both
bodies.
*/
func Message(to mail.Address) mail.Message {
	return mail.Message{
		From:    mail.Address{Name: "Sender", Email: "sender@example.com"},
		To:      []mail.Address{to},
		Subject: "Your order has shipped",
		Text:    "Hello,\n\nYour order is on its way.",
		HTML:    "<p>Hello,</p><p>Your order is on its way.</p>",
	}
}

/*
Header returns the named header of a received message, matching the
name without regard to case.
*/
func Header(msg mail.Message, name string) string {
	for key, value := range msg.Headers {
		if strings.EqualFold(key, name) {
			return value
		}
	}

	return ""
}

func runContent(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("delivers the addresses, subject and both bodies", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		to, cc := Address("Ana"), Address("Bo")
		msg := Message(to)
		msg.Cc = []mail.Address{cc}
		msg.ReplyTo = mail.Address{Name: "Help", Email: "help@example.com"}

		_, err := h.Mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		got := h.Received(t, to.Email)
		assert.Equal(t, msg.From, got.From)
		assert.Equal(t, msg.ReplyTo, got.ReplyTo)
		assert.Equal(t, []mail.Address{to}, got.To)
		assert.Equal(t, []mail.Address{cc}, got.Cc)
		assert.Equal(t, msg.Subject, got.Subject)
		assert.Equal(t, msg.Text, normalize(got.Text))
		assert.Equal(t, msg.HTML, normalize(got.HTML))
	})

	t.Run("keeps threading and list headers", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		to := Address("Ana")
		msg := Message(to)
		msg.Subject = "Re: Café ünïcode"
		msg.Headers = map[string]string{
			"Message-ID":       "<m2." + strings.ToLower(rand.Text()[:8]) + "@example.com>",
			"In-Reply-To":      "<m1@example.com>",
			"References":       "<m0@example.com> <m1@example.com>",
			"List-Unsubscribe": "<https://example.com/unsubscribe>",
			"Auto-Submitted":   "auto-replied",
		}

		_, err := h.Mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		got := h.Received(t, to.Email)
		assert.Equal(t, msg.Subject, got.Subject)
		for name, value := range msg.Headers {
			assert.Equal(t, value, Header(got, name), name)
		}
	})
}

func runAttachments(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("delivers attachments and inline images", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		to := Address("Ana")
		msg := Message(to)
		msg.HTML = `<p>Logo: <img src="cid:logo@example.com"></p>`
		msg.Attachments = []mail.Attachment{
			{Filename: "invoice.pdf", ContentType: "application/pdf", Content: []byte("%PDF-1.4 invoice")},
			{
				Filename: "logo.png", ContentType: "image/png", Content: []byte("\x89PNG\r\n\x1a\n"),
				Inline: true, ContentID: "logo@example.com",
			},
		}

		_, err := h.Mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		got := h.Received(t, to.Email)
		require.Len(t, got.Attachments, 2)
		assert.ElementsMatch(t, msg.Attachments, got.Attachments)
	})
}

func runRecipients(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("reaches bcc recipients without naming them", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		to, bcc := Address("Ana"), Address("Hidden")
		msg := Message(to)
		msg.Bcc = []mail.Address{bcc}

		_, err := h.Mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		got := h.Received(t, bcc.Email)
		assert.Equal(t, []mail.Address{to}, got.To)
		assert.Empty(t, Header(got, "Bcc"))
	})

	t.Run("sends to several recipients at once", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		first, second := Address("Ana"), Address("Bo")
		msg := Message(first)
		msg.To = append(msg.To, second)

		_, err := h.Mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		assert.Equal(t, msg.Subject, h.Received(t, first.Email).Subject)
		assert.Equal(t, msg.Subject, h.Received(t, second.Email).Subject)
	})
}

func runRaw(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("sends a prebuilt document as it was built", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		to := Address("Ana")
		built := Message(to)
		built.Headers = map[string]string{"Message-ID": "<raw." + strings.ToLower(rand.Text()[:8]) + "@example.com>"}
		raw, err := mime.NewBuilder(clock.System()).Build(built)
		require.NoError(t, err)

		_, err = h.Mailer.Send(t.Context(), mail.Message{Raw: raw})
		require.NoError(t, err)

		got := h.Received(t, to.Email)
		assert.Equal(t, built.From, got.From)
		assert.Equal(t, built.Subject, got.Subject)
		assert.Equal(t, built.Text, normalize(got.Text))
		assert.Equal(t, built.HTML, normalize(got.HTML))
		assert.Equal(t, built.Headers["Message-ID"], Header(got, "Message-ID"))
	})
}

func runRefusals(t *testing.T, newHarness NewHarness) {
	t.Helper()

	refusals := map[string]func(msg *mail.Message){
		"refuses a message without recipients": func(msg *mail.Message) { msg.To = nil },
		"refuses a message without a sender":   func(msg *mail.Message) { msg.From = mail.Address{} },
		"refuses an invalid address":           func(msg *mail.Message) { msg.To[0].Email = "not an address" },
		"refuses a header with a line break": func(msg *mail.Message) {
			msg.Headers = map[string]string{"X-Note": "a\r\nBcc: someone@example.com"}
		},
	}

	for name, change := range refusals {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			msg := Message(Address("Ana"))
			change(&msg)

			_, err := newHarness(t).Mailer.Send(t.Context(), msg)

			require.ErrorIs(t, err, mail.ErrPermanent)
		})
	}

	t.Run("stops when the context is canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := newHarness(t).Mailer.Send(ctx, Message(Address("Ana")))

		require.ErrorIs(t, err, mail.ErrTemporary)
		require.ErrorIs(t, err, context.Canceled)
	})
}

/*
normalize undoes what transports may do to a body without changing it:
CRLF line endings and a trailing line break.
*/
func normalize(body string) string {
	return strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
}
