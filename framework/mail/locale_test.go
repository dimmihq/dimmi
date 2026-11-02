package mail_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/mail"
)

/*
welcome is a mailable whose subject and body are translated in the
locale it renders in.
*/
type welcome struct {
	Name string `json:"name"`
}

func (welcome) Envelope() mail.Envelope {
	return mail.Envelope{Subject: "untranslated"}
}

func (w welcome) Content(ctx context.Context) (mail.Content, error) {
	return mail.Content{
		Subject: i18n.T(ctx, "mail.welcome_subject", nil),
		Text:    i18n.T(ctx, "mail.welcome_body", i18n.Args{"name": w.Name}),
	}, nil
}

/*
translator holds the welcome mail in English and French.
*/
func translator(t *testing.T) *i18n.Translator {
	t.Helper()

	translator, err := i18n.New(i18n.Config{Locale: "en"}, fstest.MapFS{
		"en/mail.json": {Data: []byte(`{"welcome_subject": "Welcome", "welcome_body": "Hello {{.name}}."}`)},
		"fr/mail.json": {Data: []byte(`{"welcome_subject": "Bienvenue", "welcome_body": "Bonjour {{.name}}."}`)},
	})
	require.NoError(t, err)

	return translator
}

/*
sentTo returns the subject, body and locale of the mail sent to the
address.
*/
func sentTo(t *testing.T, fake *mail.Fake, email string) (string, string, string) {
	t.Helper()

	for _, sent := range fake.Sent() {
		if sent.Message.To[0].Email == email {
			return sent.Message.Subject, sent.Message.Text, sent.Locale
		}
	}

	require.Failf(t, "no mail sent", "to %s", email)

	return "", "", ""
}

/*
drain works the queue until the fake holds the number of messages. A
job pushed a moment ago may not be due yet by the database's clock, so
draining once is not always enough.
*/
func drain(t *testing.T, q queued, fake *mail.Fake, sent int) {
	t.Helper()

	require.Eventually(t, func() bool {
		return q.queue.Drain(t.Context()) == nil && len(fake.Sent()) == sent
	}, 5*time.Second, 20*time.Millisecond)
}

func TestLocalizedMail(t *testing.T) {
	t.Parallel()

	t.Run("renders queued mail in the recipient's locale, not the sender's", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")
		fake := q.mail.Fake()
		sender := i18n.WithLocale(t.Context(), "en")

		err := q.db.Transaction(sender, func(ctx context.Context, tx *database.Tx) error {
			zoe := q.mail.To(mail.Address{Email: "zoe@example.com"}).Locale("fr")
			if err := zoe.QueueTx(ctx, tx, welcome{Name: "Zoé"}); err != nil {
				return err
			}

			return q.mail.To(mail.Address{Email: "ana@example.com"}).QueueTx(ctx, tx, welcome{Name: "Ana"})
		})
		require.NoError(t, err)
		require.NoError(t, q.mail.To(mail.Address{Email: "bo@example.com"}).Queue(t.Context(), welcome{Name: "Bo"}))
		drain(t, q, fake, 3)

		subject, body, locale := sentTo(t, fake, "zoe@example.com")
		assert.Equal(t, []string{"Bienvenue", "Bonjour Zoé.", "fr"}, []string{subject, body, locale})

		subject, body, locale = sentTo(t, fake, "ana@example.com")
		assert.Equal(t, []string{"Welcome", "Hello Ana.", "en"}, []string{subject, body, locale})

		subject, body, locale = sentTo(t, fake, "bo@example.com")
		assert.Equal(t, []string{"Welcome", "Hello Bo.", ""}, []string{subject, body, locale})
	})

	t.Run("keeps the locale of the context it was queued from by default", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")
		fake := q.mail.Fake()

		require.NoError(t, q.mail.To(mail.Address{Email: "zoe@example.com"}).
			Queue(i18n.WithLocale(t.Context(), "fr"), welcome{Name: "Zoé"}))
		drain(t, q, fake, 1)

		subject, _, locale := sentTo(t, fake, "zoe@example.com")
		assert.Equal(t, "Bienvenue", subject)
		assert.Equal(t, "fr", locale)
	})

	t.Run("renders mail sent now in the locale it names", func(t *testing.T) {
		t.Parallel()
		q := newQueued(t, "")
		fake := q.mail.Fake()

		_, err := q.mail.To(mail.Address{Email: "zoe@example.com"}).Locale("fr").
			Send(i18n.WithLocale(t.Context(), "en"), welcome{Name: "Zoé"})
		require.NoError(t, err)

		subject, body, _ := sentTo(t, fake, "zoe@example.com")
		assert.Equal(t, "Bienvenue", subject)
		assert.Equal(t, "Bonjour Zoé.", body)
	})

	t.Run("keeps the envelope subject when the content sets none", func(t *testing.T) {
		t.Parallel()
		m := mail.NewManager(mail.Config{AllowFakes: true, From: mail.Address{Email: "app@example.com"}})
		fake := m.Fake()

		_, err := m.To(mail.Address{Email: "ana@example.com"}).Send(t.Context(), invitation{Inviter: "Ana"})
		require.NoError(t, err)

		subject, _, _ := sentTo(t, fake, "ana@example.com")
		assert.Equal(t, "Ana invited you", subject)
	})
}
