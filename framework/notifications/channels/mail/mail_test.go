package mail_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/notifications"
	mailchannel "github.com/dimmihq/dimmi/framework/notifications/channels/mail"
)

type member struct {
	route any
}

func (member) NotifiableType() string { return "member" }

func (member) NotifiableID() string { return "m1" }

func (m member) RouteFor(channel string) any {
	if channel != "mail" {
		return nil
	}

	return m.route
}

type invoicePaid struct {
	Subject string
	Fail    bool
}

func (invoicePaid) Type() string { return "invoice_paid" }

func (invoicePaid) Via(notifications.Notifiable) []string { return []string{"mail"} }

func (n invoicePaid) ToMail(_ context.Context, d notifications.Delivery) (mail.Mailable, error) {
	if n.Fail {
		return nil, errors.New("template missing")
	}

	return invoiceMail{subject: n.Subject, id: d.ID}, nil
}

type invoiceMail struct {
	subject string
	id      string
}

func (m invoiceMail) Envelope() mail.Envelope {
	return mail.Envelope{Subject: m.subject, Headers: map[string]string{"X-Entity-Ref-ID": m.id}}
}

func (invoiceMail) Content(context.Context) (mail.Content, error) {
	return mail.Content{HTML: "<p>Your invoice is paid.</p>"}, nil
}

/*
silent has no mail rendering.
*/
type silent struct{}

func (silent) Type() string { return "silent" }

func (silent) Via(notifications.Notifiable) []string { return []string{"mail"} }

/*
setup returns a manager whose mail channel sends through a fake mailer.
*/
func setup(t *testing.T) (*notifications.Manager, *mail.Fake) {
	t.Helper()

	return setupChannel(t, config.Section{"driver": "mail"})
}

/*
setupChannel returns a manager whose mail channel, configured by the
section, sends through a fake mailer.
*/
func setupChannel(t *testing.T, section config.Section) (*notifications.Manager, *mail.Fake) {
	t.Helper()

	mails := mail.NewManager(mail.Config{From: mail.Address{Email: "app@example.com"}, AllowFakes: true})
	fake := mails.Fake()

	m, err := notifications.New(notifications.Config{
		Channels: map[string]config.Section{"mail": section},
	}, nil)
	require.NoError(t, err)
	m.Extend("mail", mailchannel.Factory(mails))

	return m, fake
}

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("sends the mailable to the address the recipient routes mail to", func(t *testing.T) {
		t.Parallel()
		m, fake := setup(t)

		require.NoError(t, m.Send(t.Context(), invoicePaid{Subject: "Paid"}, member{route: "Ada <ada@example.com>"}))

		sent := fake.Sent()
		require.Len(t, sent, 1)
		assert.Equal(t, []mail.Address{{Name: "Ada", Email: "ada@example.com"}}, sent[0].Message.To)
		assert.Equal(t, "Paid", sent[0].Message.Subject)
		assert.Equal(t, "app@example.com", sent[0].Message.From.Email)
		assert.Contains(t, sent[0].Message.Text, "Your invoice is paid.")
		assert.NotEmpty(t, sent[0].Message.Headers["X-Entity-Ref-ID"])
		mail.AssertSent(t, fake, func(invoiceMail) bool { return true })
	})

	t.Run("reads addresses and lists of either", func(t *testing.T) {
		t.Parallel()
		ada := mail.Address{Email: "ada@example.com"}
		bo := mail.Address{Email: "bo@example.com"}

		routes := []any{ada, []mail.Address{ada, bo}, []string{"ada@example.com", "", "bo@example.com"}}
		want := [][]mail.Address{{ada}, {ada, bo}, {ada, bo}}

		for i, route := range routes {
			m, fake := setup(t)

			require.NoError(t, m.Send(t.Context(), invoicePaid{Subject: "Paid"}, member{route: route}))

			require.Len(t, fake.Sent(), 1)
			assert.Equal(t, want[i], fake.Sent()[0].Message.To)
		}
	})

	t.Run("sends on the mailer its options name", func(t *testing.T) {
		t.Parallel()
		m, fake := setupChannel(t, config.Section{"driver": "mail", "mailer": "customer"})

		require.NoError(t, m.Send(t.Context(), invoicePaid{Subject: "Paid"}, member{route: "ada@example.com"}))

		assert.Equal(t, "customer", fake.Sent()[0].Mailer)
	})

	t.Run("refuses options it cannot read", func(t *testing.T) {
		t.Parallel()
		m, _ := setupChannel(t, config.Section{"driver": "mail", "mailer": []int{1}})

		err := m.Send(t.Context(), invoicePaid{Subject: "Paid"}, member{route: "ada@example.com"})

		require.ErrorContains(t, err, "mail channel mail")
	})

	t.Run("skips a recipient without an address", func(t *testing.T) {
		t.Parallel()
		m, fake := setup(t)

		require.NoError(t, m.Send(t.Context(), invoicePaid{}, member{}, member{route: ""}, member{route: []string{}}))

		fake.AssertNothingSent(t)
	})

	t.Run("refuses a route it cannot read", func(t *testing.T) {
		t.Parallel()
		m, fake := setup(t)

		err := m.Send(t.Context(), invoicePaid{}, member{route: 42}, member{route: "not an address"})

		require.ErrorIs(t, err, notifications.ErrInvalidRoute)
		require.ErrorContains(t, err, "mail route int")
		require.ErrorContains(t, err, "mail: invalid address")
		fake.AssertNothingSent(t)
	})

	t.Run("refuses a notification without a mail rendering", func(t *testing.T) {
		t.Parallel()
		m, _ := setup(t)

		err := m.Send(t.Context(), silent{}, member{route: "ada@example.com"})

		require.ErrorIs(t, err, notifications.ErrUnsupported)
	})

	t.Run("returns a rendering failure", func(t *testing.T) {
		t.Parallel()
		m, _ := setup(t)

		err := m.Send(t.Context(), invoicePaid{Fail: true}, member{route: "ada@example.com"})

		require.ErrorContains(t, err, "render mail invoice_paid: template missing")
	})

	t.Run("marks a message the mailer refuses for good as permanent", func(t *testing.T) {
		t.Parallel()
		m, _ := setup(t)

		err := m.Send(t.Context(), invoicePaid{Subject: "Paid\nBcc: x@example.com"}, member{route: "ada@example.com"})

		require.ErrorIs(t, err, notifications.ErrPermanent)
		require.ErrorIs(t, err, mail.ErrPermanent)
	})

	t.Run("leaves a temporary failure retryable", func(t *testing.T) {
		t.Parallel()
		m, _ := setup(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := m.Send(ctx, invoicePaid{Subject: "Paid"}, member{route: "ada@example.com"})

		require.ErrorIs(t, err, mail.ErrTemporary)
		require.NotErrorIs(t, err, notifications.ErrPermanent)
	})
}
