package mail_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
	"github.com/dimmihq/dimmi/framework/mail/mime"
)

func newFakeHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	m := mail.NewManager(mail.Config{AllowFakes: true})
	fake := m.Fake()
	mailer, err := m.Mailer("customer")
	require.NoError(t, err)

	return drivertest.Harness{
		Mailer: mailer,
		Received: func(t *testing.T, recipient string) mail.Message {
			t.Helper()

			for _, sent := range fake.Sent() {
				msg, err := mime.Decompose(sent.Message)
				require.NoError(t, err)

				recipients, err := msg.Recipients()
				require.NoError(t, err)

				if slices.ContainsFunc(recipients, func(a mail.Address) bool { return a.Email == recipient }) {
					msg.Bcc = nil

					return msg
				}
			}

			t.Fatalf("no message sent to %s", recipient)

			return mail.Message{}
		},
	}
}

func TestFake(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newFakeHarness)
}

/*
failures records assertion failures instead of failing the test.
*/
type failures struct {
	testing.TB
	errors []string
}

func (f *failures) Helper() {}

func (f *failures) Errorf(format string, _ ...any) {
	f.errors = append(f.errors, format)
}

func TestFakeAssertions(t *testing.T) {
	t.Parallel()

	t.Run("fails when nothing matches", func(t *testing.T) {
		t.Parallel()
		fake := mail.NewManager(mail.Config{AllowFakes: true}).Fake()
		tb := &failures{TB: t}

		fake.AssertMessage(tb, func(mail.SentMessage) bool { return true })
		mail.AssertSent(tb, fake, func(greeting) bool { return true })
		fake.AssertNothingSent(tb)

		assert.Len(t, tb.errors, 2)
	})

	t.Run("fails when something was sent", func(t *testing.T) {
		t.Parallel()
		m := mail.NewManager(mail.Config{AllowFakes: true})
		fake := m.Fake()
		mailer, err := m.Mailer("system")
		require.NoError(t, err)
		send(t, mailer)
		tb := &failures{TB: t}

		fake.AssertNothingSent(tb)
		mail.AssertSent(tb, fake, func(greeting) bool { return true })

		assert.Len(t, tb.errors, 2)
	})

	t.Run("numbers what it records", func(t *testing.T) {
		t.Parallel()
		m := mail.NewManager(mail.Config{AllowFakes: true})
		m.Fake()
		mailer, err := m.Mailer("system")
		require.NoError(t, err)

		assert.Equal(t, "fake-1", send(t, mailer))
		assert.Equal(t, "fake-2", send(t, mailer))
	})
}
