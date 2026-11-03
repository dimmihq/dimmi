package log_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/mail"
	maillog "github.com/dimmihq/dimmi/framework/mail/drivers/log"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
	"github.com/dimmihq/dimmi/framework/mail/mime"
)

/*
line is one log line the mailer writes.
*/
type line struct {
	Level      string `json:"level"`
	Msg        string `json:"msg"`
	Mailer     string `json:"mailer"`
	Fallback   bool   `json:"mail_fallback"`
	Recipients string `json:"recipients"`
	Subject    string `json:"subject"`
	Links      string `json:"links"`
	Message    string `json:"message"`
}

func newMailer(t *testing.T) (mail.Mailer, *bytes.Buffer) {
	t.Helper()

	return newMailerWith(t, config.Section{"driver": "log"})
}

func newMailerWith(t *testing.T, section config.Section) (mail.Mailer, *bytes.Buffer) {
	t.Helper()

	var out bytes.Buffer
	logger, err := log.New(log.Options{Driver: "json", Level: "info", Writer: &out})
	require.NoError(t, err)

	mailer, err := maillog.Factory(logger, clock.System())("system", section)
	require.NoError(t, err)

	return mailer, &out
}

func lines(t *testing.T, out *bytes.Buffer) []line {
	t.Helper()

	var all []line
	scanner := bufio.NewScanner(bytes.NewReader(out.Bytes()))
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		var l line
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &l))
		all = append(all, l)
	}

	return all
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	mailer, out := newMailer(t)

	return drivertest.Harness{
		Mailer: mailer,
		Received: func(t *testing.T, recipient string) mail.Message {
			t.Helper()

			for _, l := range lines(t, out) {
				if strings.Contains(l.Recipients, recipient) {
					msg, err := mime.Decompose(mail.Message{Raw: []byte(l.Message)})
					require.NoError(t, err)

					return msg
				}
			}

			t.Fatalf("no line for %s", recipient)

			return mail.Message{}
		},
	}
}

func TestMailer(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestLine(t *testing.T) {
	t.Parallel()

	t.Run("writes the whole document with the mailer name", func(t *testing.T) {
		t.Parallel()
		mailer, out := newMailer(t)
		msg := drivertest.Message(drivertest.Address("Ana"))
		msg.Text = "Sign in: https://example.com/sign-in?code=abc"

		_, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		written := lines(t, out)
		require.Len(t, written, 1)
		assert.Equal(t, "mail logged instead of sent", written[0].Msg)
		assert.Equal(t, "INFO", written[0].Level)
		assert.Equal(t, "system", written[0].Mailer)
		assert.True(t, written[0].Fallback)
		assert.Equal(t, msg.Subject, written[0].Subject)
		assert.Equal(t, "https://example.com/sign-in?code=abc", written[0].Links)
		assert.Contains(t, written[0].Message, "https://example.com/sign-in?code=3Dabc")
	})

	t.Run("writes only the subject, recipients and links when asked, at the level asked", func(t *testing.T) {
		t.Parallel()
		mailer, out := newMailerWith(t, config.Section{"driver": "log", "level": "warn", "content": "links"})
		msg := drivertest.Message(drivertest.Address("Ana"))
		msg.Text = "Open https://example.com/a#token=1 or https://example.com/b, then https://example.com/a#token=1"

		_, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		written := lines(t, out)
		require.Len(t, written, 1)
		assert.Equal(t, "WARN", written[0].Level)
		assert.Equal(t, "https://example.com/a#token=1 https://example.com/b", written[0].Links)
		assert.Empty(t, written[0].Message)
	})

	t.Run("finds the links of a message that has only HTML", func(t *testing.T) {
		t.Parallel()
		mailer, out := newMailerWith(t, config.Section{"driver": "log", "content": "links"})
		msg := drivertest.Message(drivertest.Address("Ana"))
		msg.Text, msg.HTML = "", `<a href="https://example.com/r#id=1&amp;token=2">Reset</a>`

		_, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		assert.Equal(t, "https://example.com/r#id=1&token=2", lines(t, out)[0].Links)
	})

	t.Run("keeps every link and the body out of the line when only the envelope is wanted", func(t *testing.T) {
		t.Parallel()
		mailer, out := newMailerWith(t, config.Section{"driver": "log", "content": "envelope"})
		msg := drivertest.Message(drivertest.Address("Ana"))
		msg.Text = "Sign in: https://example.com/sign-in#token=secret"

		_, err := mailer.Send(t.Context(), msg)
		require.NoError(t, err)

		assert.NotContains(t, out.String(), "secret")
		assert.Equal(t, msg.Subject, lines(t, out)[0].Subject)
	})

	t.Run("refuses a level or content it does not know", func(t *testing.T) {
		t.Parallel()
		logger, err := log.New(log.Options{Driver: "json", Level: "info", Writer: &bytes.Buffer{}})
		require.NoError(t, err)

		for _, section := range []config.Section{
			{"driver": "log", "level": "loud"},
			{"driver": "log", "content": "everything"},
			{"driver": "log", "level": []int{1}},
		} {
			_, err := maillog.Factory(logger, clock.System())("system", section)
			require.Error(t, err)
		}

		_, err = maillog.NewWith(logger, clock.System(), "system", maillog.Options{Level: "info", Content: "all"})
		require.ErrorIs(t, err, maillog.ErrInvalidOption)
	})

	t.Run("refuses a raw message with broken recipients", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t)

		_, err := mailer.Send(t.Context(), mail.Message{Raw: []byte("From: a@example.com\r\nTo: <<x\r\n\r\nbody")})

		require.ErrorIs(t, err, mail.ErrPermanent)
	})
}
