package smtp_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"maps"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/drivers/smtp"
	"github.com/dimmihq/dimmi/framework/mail/drivertest"
	"github.com/dimmihq/dimmi/framework/mail/mime"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
section returns the mailer configuration for the Mailpit server from the
environment, which offers no encryption.
*/
func section(t *testing.T) config.Section {
	t.Helper()

	server, err := url.Parse(testkit.Env(t, "MAIL_SMTP_URL"))
	require.NoError(t, err)
	port, err := strconv.Atoi(server.Port())
	require.NoError(t, err)

	return config.Section{
		"driver":         "smtp",
		"host":           server.Hostname(),
		"port":           port,
		"allow_insecure": true,
		"helo":           "test.example.com",
	}
}

func newMailer(t *testing.T, extra config.Section) mail.Mailer {
	t.Helper()

	cfg := section(t)
	maps.Copy(cfg, extra)

	mailer, err := smtp.Factory(clock.System())("system", cfg)
	require.NoError(t, err)

	return mailer
}

/*
raw returns the document Mailpit received for the recipient, read from
its HTTP API.
*/
func raw(t *testing.T, recipient string) []byte {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true})
	require.NoError(t, err)
	api := testkit.Env(t, "MAILPIT_API_URL")

	query := url.Values{"query": {`addressed:"` + recipient + `"`}}
	resp, err := client.New().Get(t.Context(), api+"/api/v1/search?"+query.Encode())
	require.NoError(t, err)

	var found struct{ Messages []struct{ ID string } }
	require.NoError(t, resp.JSON(&found))
	require.Len(t, found.Messages, 1, "messages for %s", recipient)

	resp, err = client.New().Get(t.Context(), api+"/api/v1/message/"+found.Messages[0].ID+"/raw")
	require.NoError(t, err)

	return resp.Body
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	return drivertest.Harness{
		Mailer: newMailer(t, nil),
		Received: func(t *testing.T, recipient string) mail.Message {
			t.Helper()

			msg, err := mime.Decompose(mail.Message{Raw: raw(t, recipient)})
			require.NoError(t, err)

			return msg
		},
	}
}

func TestMailer(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestDKIM(t *testing.T) {
	t.Parallel()

	t.Run("signs every message with the configured key", func(t *testing.T) {
		t.Parallel()
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		require.NoError(t, err)
		mailer := newMailer(t, config.Section{
			"dkim_domain": "example.com", "dkim_selector": "s1", "dkim_private_key": string(private),
		})
		to := drivertest.Address("Ana")

		_, err = mailer.Send(t.Context(), drivertest.Message(to))
		require.NoError(t, err)

		verifications, err := dkim.VerifyWithOptions(bytes.NewReader(raw(t, to.Email)), &dkim.VerifyOptions{
			LookupTXT: func(string) ([]string, error) {
				return []string{"v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(public)}, nil
			},
		})
		require.NoError(t, err)
		require.Len(t, verifications, 1)
		require.NoError(t, verifications[0].Err)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("reports the queue id from the server reply", func(t *testing.T) {
		t.Parallel()

		result, err := newMailer(t, nil).Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.NoError(t, err)
		assert.NotEmpty(t, result.ProviderMessageID)
	})

	t.Run("requires a host", func(t *testing.T) {
		t.Parallel()

		_, err := smtp.New(clock.System(), smtp.Options{})

		require.ErrorIs(t, err, smtp.ErrHostRequired)
	})

	t.Run("refuses an invalid dkim key", func(t *testing.T) {
		t.Parallel()

		_, err := smtp.Factory(clock.System())("system", config.Section{"host": "localhost", "dkim_private_key": "x"})

		require.Error(t, err)
	})

	t.Run("rejects a malformed section", func(t *testing.T) {
		t.Parallel()

		_, err := smtp.Factory(clock.System())("system", config.Section{"port": []int{1}})

		require.Error(t, err)
	})

	t.Run("refuses to send in plain text unless allowed", func(t *testing.T) {
		t.Parallel()
		mailer := newMailer(t, config.Section{"allow_insecure": false})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.ErrorIs(t, err, mail.ErrTemporary)
	})

	t.Run("expects implicit tls when asked", func(t *testing.T) {
		t.Parallel()
		mailer := newMailer(t, config.Section{"tls": true, "timeout": "2s"})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.ErrorIs(t, err, mail.ErrTemporary)
	})

	t.Run("logs in when given a username", func(t *testing.T) {
		t.Parallel()
		mailer := newMailer(t, config.Section{"username": "u", "password": "p"})

		_, err := mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.ErrorIs(t, err, mail.ErrTemporary)
	})

	t.Run("reports an unreachable server as temporary", func(t *testing.T) {
		t.Parallel()
		mailer, err := smtp.New(clock.System(), smtp.Options{Host: "127.0.0.1", Port: 1, Timeout: time.Second})
		require.NoError(t, err)

		_, err = mailer.Send(t.Context(), drivertest.Message(drivertest.Address("Ana")))

		require.ErrorIs(t, err, mail.ErrTemporary)
	})
}
