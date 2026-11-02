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
	Msg        string `json:"msg"`
	Mailer     string `json:"mailer"`
	Recipients string `json:"recipients"`
	Message    string `json:"message"`
}

func newMailer(t *testing.T) (mail.Mailer, *bytes.Buffer) {
	t.Helper()

	var out bytes.Buffer
	logger, err := log.New(log.Options{Driver: "json", Level: "info", Writer: &out})
	require.NoError(t, err)

	mailer, err := maillog.Factory(logger, clock.System())("system", config.Section{"driver": "log"})
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
		assert.Equal(t, "mail sent", written[0].Msg)
		assert.Equal(t, "system", written[0].Mailer)
		assert.Contains(t, written[0].Message, "https://example.com/sign-in?code=3Dabc")
	})

	t.Run("refuses a raw message with broken recipients", func(t *testing.T) {
		t.Parallel()
		mailer, _ := newMailer(t)

		_, err := mailer.Send(t.Context(), mail.Message{Raw: []byte("From: a@example.com\r\nTo: <<x\r\n\r\nbody")})

		require.ErrorIs(t, err, mail.ErrPermanent)
	})
}
