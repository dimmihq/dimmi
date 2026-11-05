package sendgrid_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/sendgrid"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
)

const timestamp = "1800000000"

func newKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)

	return key, base64.StdEncoding.EncodeToString(der)
}

func multipartBody(t *testing.T, fields map[string]string) ([]byte, string) {
	t.Helper()

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		require.NoError(t, writer.WriteField(name, value))
	}
	require.NoError(t, writer.Close())

	return body.Bytes(), writer.FormDataContentType()
}

func signed(t *testing.T, key *ecdsa.PrivateKey, fields map[string]string) drivertest.Post {
	t.Helper()

	body, contentType := multipartBody(t, fields)

	digest := sha256.New()
	digest.Write([]byte(timestamp))
	digest.Write(body)

	signature, err := ecdsa.SignASN1(rand.Reader, key, digest.Sum(nil))
	require.NoError(t, err)

	return drivertest.Post{
		Header: http.Header{
			"Content-Type":           {contentType},
			sendgrid.HeaderSignature: {base64.StdEncoding.EncodeToString(signature)},
			sendgrid.HeaderTimestamp: {timestamp},
		},
		Body: body,
	}
}

func fields(t *testing.T, d drivertest.Delivery) map[string]string {
	t.Helper()

	envelope, err := json.Marshal(map[string]any{"to": d.Recipients, "from": d.EnvelopeFrom})
	require.NoError(t, err)

	return map[string]string{
		"email": string(d.Raw), "envelope": string(envelope), "SPF": "pass",
		"dkim": "{@sender.example : pass}", "spam_score": "0.1",
	}
}

func newSource(t *testing.T) (*sendgrid.Source, *ecdsa.PrivateKey) {
	t.Helper()

	key, public := newKey(t)
	source, err := sendgrid.New(clock.NewFake(time.Unix(1_800_000_000, 0)), sendgrid.Options{
		PublicKey: public, MaxWebhookBytes: 8192,
	})
	require.NoError(t, err)

	return source, key
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	source, key := newSource(t)
	other, _ := newKey(t)

	return drivertest.Harness{
		Source:   source,
		MaxBytes: 8192,
		ExpectedID: func(d drivertest.Delivery) string {
			return hashing.SHA256Hex(d.Raw)[:32]
		},
		Payload: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return signed(t, key, fields(t, d))
		},
		Forged: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return signed(t, other, fields(t, d))
		},
	}
}

func TestSource(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestDelivery(t *testing.T) {
	t.Parallel()

	t.Run("reads the verdicts sendgrid posts beside the message", func(t *testing.T) {
		t.Parallel()
		source, key := newSource(t)

		env, err := source.Receive(t.Context(), signed(t, key, fields(t, drivertest.NewDelivery())).Delivery())
		require.NoError(t, err)

		assert.Equal(t, mailbox.Verdicts{
			SPF: mailbox.VerdictPass, DKIM: mailbox.VerdictPass, Spam: mailbox.VerdictPass,
		}, env.Verdicts)
	})

	t.Run("maps every dkim and spam answer", func(t *testing.T) {
		t.Parallel()
		source, key := newSource(t)

		cases := []struct {
			dkim, spam string
			want       mailbox.Verdicts
		}{
			{"none", "9.5", mailbox.Verdicts{
				SPF: mailbox.VerdictPass, DKIM: mailbox.VerdictNone, Spam: mailbox.VerdictFail,
			}},
			{"{@x : fail}", "", mailbox.Verdicts{SPF: mailbox.VerdictPass, DKIM: mailbox.VerdictFail}},
			{"odd", "n/a", mailbox.Verdicts{SPF: mailbox.VerdictPass, DKIM: mailbox.VerdictUnknown}},
		}

		for _, c := range cases {
			values := fields(t, drivertest.NewDelivery())
			values["dkim"], values["spam_score"] = c.dkim, c.spam

			env, err := source.Receive(t.Context(), signed(t, key, values).Delivery())
			require.NoError(t, err)
			assert.Equal(t, c.want, env.Verdicts)
		}
	})

	t.Run("names the setting when the raw message is left out", func(t *testing.T) {
		t.Parallel()
		source, key := newSource(t)
		values := fields(t, drivertest.NewDelivery())
		delete(values, "email")

		_, err := source.Receive(t.Context(), signed(t, key, values).Delivery())

		var misconfigured *mailbox.MisconfiguredError
		require.ErrorAs(t, err, &misconfigured)
	})

	t.Run("refuses a body rewritten after signing", func(t *testing.T) {
		t.Parallel()
		source, key := newSource(t)
		post := signed(t, key, fields(t, drivertest.NewDelivery()))
		post.Body = append(post.Body, '\n')

		_, err := source.Receive(t.Context(), post.Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("refuses a signature that is not base64", func(t *testing.T) {
		t.Parallel()
		source, key := newSource(t)
		post := signed(t, key, fields(t, drivertest.NewDelivery()))
		post.Header.Set(sendgrid.HeaderSignature, "%%%")

		_, err := source.Receive(t.Context(), post.Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("refuses an envelope that cannot be read", func(t *testing.T) {
		t.Parallel()
		source, key := newSource(t)
		values := fields(t, drivertest.NewDelivery())
		values["envelope"] = "nope"

		_, err := source.Receive(t.Context(), signed(t, key, values).Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("refuses a broken multipart body that is signed", func(t *testing.T) {
		t.Parallel()
		source, key := newSource(t)
		post := signed(t, key, map[string]string{"a": "b"})
		body := post.Body[:len(post.Body)-10]

		digest := sha256.New()
		digest.Write([]byte(timestamp))
		digest.Write(body)
		signature, err := ecdsa.SignASN1(rand.Reader, key, digest.Sum(nil))
		require.NoError(t, err)

		post.Body = body
		post.Header.Set(sendgrid.HeaderSignature, base64.StdEncoding.EncodeToString(signature))

		_, err = source.Receive(t.Context(), post.Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})
}

func TestFactory(t *testing.T) {
	t.Parallel()

	t.Run("refuses to start without its public key and names it", func(t *testing.T) {
		t.Parallel()

		_, err := sendgrid.Factory(clock.System())("sendgrid", config.Section{})

		require.ErrorIs(t, err, mailbox.ErrMissingOption)
		assert.Contains(t, err.Error(), "mailbox.sources.sendgrid.public_key")
	})

	t.Run("refuses a public key that is not base64", func(t *testing.T) {
		t.Parallel()

		_, err := sendgrid.Factory(clock.System())("sendgrid", config.Section{"public_key": "%%%"})

		require.Error(t, err)
	})

	t.Run("refuses bytes that are no key", func(t *testing.T) {
		t.Parallel()

		_, err := sendgrid.Factory(clock.System())("sendgrid", config.Section{"public_key": "aGVsbG8="})

		require.Error(t, err)
	})

	t.Run("refuses a key that is not ecdsa", func(t *testing.T) {
		t.Parallel()
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		require.NoError(t, err)

		_, err = sendgrid.Factory(clock.System())("sendgrid", config.Section{
			"public_key": base64.StdEncoding.EncodeToString(der),
		})

		require.Error(t, err)
	})

	t.Run("builds a source from its options", func(t *testing.T) {
		t.Parallel()
		_, public := newKey(t)

		source, err := sendgrid.Factory(clock.System())("sendgrid", config.Section{"public_key": public})
		require.NoError(t, err)

		assert.Equal(t, "sendgrid", source.Name())
	})
}
