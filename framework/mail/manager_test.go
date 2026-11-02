package mail_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
recorder is a driver that keeps what it was asked to send, standing in
for a provider in manager tests.
*/
type recorder struct {
	name string
	sent []mail.Message
}

func (r *recorder) Send(_ context.Context, msg mail.Message) (mail.SendResult, error) {
	r.sent = append(r.sent, msg)

	return mail.SendResult{ProviderMessageID: r.name}, nil
}

func newManager(t *testing.T) *mail.Manager {
	t.Helper()

	m := mail.NewManager(mail.Config{
		Default:    "system",
		AllowFakes: true,
		From:       mail.Address{Name: "App", Email: "app@example.com"},
		Mailers: map[string]config.Section{
			"system":   {"driver": "recorder"},
			"customer": {"driver": "recorder"},
			"broken":   {"driver": "missing"},
		},
	})
	m.Extend("recorder", func(name string, _ config.Section) (mail.Mailer, error) {
		return &recorder{name: name}, nil
	})

	return m
}

func send(t *testing.T, mailer mail.Mailer) string {
	t.Helper()

	result, err := mailer.Send(t.Context(), mail.Message{
		From: mail.Address{Email: "a@example.com"}, To: []mail.Address{{Email: "b@example.com"}},
	})
	require.NoError(t, err)

	return result.ProviderMessageID
}

func TestManager(t *testing.T) {
	t.Parallel()

	t.Run("resolves mailers by name", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)

		customer, err := m.Mailer("customer")
		require.NoError(t, err)
		def, err := m.Default()
		require.NoError(t, err)

		assert.Equal(t, "customer", send(t, customer))
		assert.Equal(t, "system", send(t, def))
	})

	t.Run("resolves an unconfigured name to the default mailer", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)

		mailer, err := m.Mailer("marketing")
		require.NoError(t, err)

		assert.Equal(t, "system", send(t, mailer))
	})

	t.Run("reports a mailer whose driver is not registered", func(t *testing.T) {
		t.Parallel()

		_, err := newManager(t).Mailer("broken")

		require.ErrorIs(t, err, manager.ErrUnknownDriver)
	})

	t.Run("refuses a fake unless fakes are allowed", func(t *testing.T) {
		t.Parallel()
		m := mail.NewManager(mail.Config{})

		assert.PanicsWithValue(t, mail.ErrFakesNotAllowed, func() { m.Fake() })
	})

	t.Run("routes every mailer to the fake once faked", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)
		fake := m.Fake()

		customer, err := m.Mailer("customer")
		require.NoError(t, err)
		send(t, customer)

		fake.AssertMessage(t, func(sent mail.SentMessage) bool { return sent.Mailer == "customer" })
	})
}

/*
greeting is a mailable with fixed content.
*/
type greeting struct {
	name    string
	content mail.Content
	err     error
}

func (g greeting) Envelope() mail.Envelope {
	return mail.Envelope{
		Subject: "Hello " + g.name,
		ReplyTo: mail.Address{Email: "help@example.com"},
		Headers: map[string]string{"Auto-Submitted": "auto-generated"},
		Tags:    map[string]string{"kind": "greeting"},
	}
}

func (g greeting) Content(context.Context) (mail.Content, error) {
	return g.content, g.err
}

/*
page is a view rendering fixed HTML, or failing.
*/
type page struct {
	html string
	err  error
}

func (p page) Render(_ context.Context, w io.Writer) error {
	if p.err != nil {
		return p.err
	}

	_, err := io.WriteString(w, p.html)

	return err
}

func TestPendingMail(t *testing.T) {
	t.Parallel()

	ana := mail.Address{Email: "ana@example.com"}

	t.Run("sends a rendered mailable on the default mailer", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)
		fake := m.Fake()
		mailable := greeting{name: "Ana", content: mail.View(page{html: `<p>Hi <a href="https://example.com">there</a></p>`})}

		_, err := m.To(ana).Cc(mail.Address{Email: "bo@example.com"}).Bcc(mail.Address{Email: "cy@example.com"}).
			Send(t.Context(), mailable)
		require.NoError(t, err)

		sent := fake.Sent()
		require.Len(t, sent, 1)
		assert.Equal(t, "system", sent[0].Mailer)
		assert.Equal(t, mailable, sent[0].Mailable)
		msg := sent[0].Message
		assert.Equal(t, mail.Address{Name: "App", Email: "app@example.com"}, msg.From)
		assert.Equal(t, "Hello Ana", msg.Subject)
		assert.Equal(t, []mail.Address{ana}, msg.To)
		assert.Equal(t, "bo@example.com", msg.Cc[0].Email)
		assert.Equal(t, "cy@example.com", msg.Bcc[0].Email)
		assert.Equal(t, `<p>Hi <a href="https://example.com">there</a></p>`, msg.HTML)
		assert.Equal(t, "Hi there <https://example.com>", msg.Text)
		assert.Equal(t, "auto-generated", msg.Headers["Auto-Submitted"])
		assert.Equal(t, "greeting", msg.Tags["kind"])
		mail.AssertSent(t, fake, func(g greeting) bool { return g.name == "Ana" })
	})

	t.Run("sends on the mailer it names", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)

		result, err := m.To(ana).Mailer("customer").Send(t.Context(), greeting{content: mail.Content{Text: "hi"}})
		require.NoError(t, err)
		assert.Equal(t, "customer", result.ProviderMessageID)

		fake := m.Fake()
		_, err = m.To(ana).Mailer("customer").Send(t.Context(), greeting{content: mail.Content{Text: "hi"}})
		require.NoError(t, err)
		assert.Equal(t, "customer", fake.Sent()[0].Mailer)
	})

	t.Run("keeps a text body the content sets", func(t *testing.T) {
		t.Parallel()
		m := newManager(t)
		fake := m.Fake()

		_, err := m.To(ana).Send(t.Context(), greeting{content: mail.Content{Text: "plain", HTML: "<b>rich</b>"}})
		require.NoError(t, err)

		assert.Equal(t, "plain", fake.Sent()[0].Message.Text)
	})

	t.Run("sends through the real default mailer when not faked", func(t *testing.T) {
		t.Parallel()

		result, err := newManager(t).To(ana).Send(t.Context(), greeting{content: mail.Content{Text: "hi"}})

		require.NoError(t, err)
		assert.Equal(t, "system", result.ProviderMessageID)
	})

	t.Run("reports content that fails", func(t *testing.T) {
		t.Parallel()
		errContent := errors.New("no data")

		_, err := newManager(t).To(ana).Send(t.Context(), greeting{err: errContent})

		require.ErrorIs(t, err, errContent)
	})

	t.Run("reports a view that fails to render", func(t *testing.T) {
		t.Parallel()
		errRender := errors.New("template broke")

		_, err := newManager(t).To(ana).Send(t.Context(), greeting{content: mail.View(page{err: errRender})})

		require.ErrorIs(t, err, errRender)
	})

	t.Run("reports a default mailer that cannot be built", func(t *testing.T) {
		t.Parallel()
		m := mail.NewManager(mail.Config{Default: "system"})

		_, err := m.To(ana).Send(t.Context(), greeting{content: mail.Content{Text: "hi"}})

		require.ErrorIs(t, err, manager.ErrUnknownInstance)
	})
}
