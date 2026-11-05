package mailgun_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/mailgun"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
	"github.com/dimmihq/dimmi/framework/testkit"
)

const signingKey = "key-signing"

func now() time.Time {
	return time.Unix(1_800_000_000, 0)
}

func newSource(t *testing.T) *mailgun.Source {
	t.Helper()

	store, err := cache.Open(testkit.NewValkey(t), cache.Options{Keyspace: "cache"})
	require.NoError(t, err)
	t.Cleanup(store.Close)

	return mailgun.New(clock.NewFake(now()), mailbox.NewReplays(store), mailgun.Options{
		SigningKey: signingKey, MaxWebhookBytes: 8192,
	})
}

func sign(key, timestamp, token string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(timestamp + token))

	return hex.EncodeToString(mac.Sum(nil))
}

func form(key string, at time.Time, d drivertest.Delivery) url.Values {
	timestamp := strconv.FormatInt(at.Unix(), 10)
	token := strings.ToLower(rand.Text())

	return url.Values{
		"timestamp": {timestamp}, "token": {token}, "signature": {sign(key, timestamp, token)},
		"recipient": {strings.Join(d.Recipients, ",")}, "sender": {d.EnvelopeFrom}, "body-mime": {string(d.Raw)},
	}
}

func post(values url.Values) drivertest.Post {
	return drivertest.Post{
		Header: http.Header{"Content-Type": {"application/x-www-form-urlencoded"}},
		Body:   []byte(values.Encode()),
	}
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	return drivertest.Harness{
		Source:   newSource(t),
		MaxBytes: 8192,
		Keyed:    true,
		ExpectedID: func(d drivertest.Delivery) string {
			return hashing.SHA256Hex(d.Raw)[:32]
		},
		Payload: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return post(form(signingKey, now(), d))
		},
		Forged: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return post(form("other-key", now(), d))
		},
	}
}

func TestSource(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestDelivery(t *testing.T) {
	t.Parallel()

	t.Run("believes only the first of each verdict header", func(t *testing.T) {
		t.Parallel()
		d := drivertest.NewDelivery()
		d.Raw = append([]byte("X-Mailgun-Spf: Pass\r\nX-Mailgun-Dkim-Check-Result: Fail\r\nX-Mailgun-Sflag: Yes\r\n"+
			"X-Mailgun-Spf: Fail\r\nX-Mailgun-Sflag: No\r\n"), d.Raw...)

		env, err := newSource(t).Receive(t.Context(), post(form(signingKey, now(), d)).Delivery())
		require.NoError(t, err)

		assert.Equal(t, mailbox.Verdicts{
			SPF: mailbox.VerdictPass, DKIM: mailbox.VerdictFail, Spam: mailbox.VerdictFail,
		}, env.Verdicts)
	})

	t.Run("reads a multipart post", func(t *testing.T) {
		t.Parallel()
		d := drivertest.NewDelivery()

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for name, values := range form(signingKey, now(), d) {
			require.NoError(t, writer.WriteField(name, values[0]))
		}
		require.NoError(t, writer.Close())

		env, err := newSource(t).Receive(t.Context(), drivertest.Post{
			Header: http.Header{"Content-Type": {writer.FormDataContentType()}}, Body: body.Bytes(),
		}.Delivery())
		require.NoError(t, err)

		assert.Equal(t, d.Raw, drivertest.Read(t, env))
	})

	t.Run("refuses a signature older than five minutes", func(t *testing.T) {
		t.Parallel()

		_, err := newSource(t).Receive(t.Context(),
			post(form(signingKey, now().Add(-6*time.Minute), drivertest.NewDelivery())).Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("refuses a signature from the future", func(t *testing.T) {
		t.Parallel()

		_, err := newSource(t).Receive(t.Context(),
			post(form(signingKey, now().Add(6*time.Minute), drivertest.NewDelivery())).Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("refuses a signature that is not hex", func(t *testing.T) {
		t.Parallel()
		values := form(signingKey, now(), drivertest.NewDelivery())
		values.Set("signature", "zz")

		_, err := newSource(t).Receive(t.Context(), post(values).Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("refuses a signed post without the raw message", func(t *testing.T) {
		t.Parallel()
		values := form(signingKey, now(), drivertest.NewDelivery())
		values.Del("body-mime")

		_, err := newSource(t).Receive(t.Context(), post(values).Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("refuses a form that cannot be read", func(t *testing.T) {
		t.Parallel()

		_, err := newSource(t).Receive(t.Context(), drivertest.Post{Body: []byte("%zz")}.Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("tells mailgun to stop retrying only a refused delivery", func(t *testing.T) {
		t.Parallel()
		s := newSource(t)

		assert.Equal(t, http.StatusNotAcceptable, mailbox.Status(s, mailbox.ErrRejected))
		assert.Equal(t, http.StatusServiceUnavailable, mailbox.Status(s, errors.New("disk down")))
	})
}

func TestFactory(t *testing.T) {
	t.Parallel()

	t.Run("refuses to start without its signing key and names it", func(t *testing.T) {
		t.Parallel()

		_, err := mailgun.Factory(clock.System(), nil)("mailgun", config.Section{})

		require.ErrorIs(t, err, mailbox.ErrMissingOption)
		assert.Contains(t, err.Error(), "mailbox.sources.mailgun.signing_key")
	})

	t.Run("builds a source from its options", func(t *testing.T) {
		t.Parallel()

		source, err := mailgun.Factory(clock.System(), nil)("mailgun", config.Section{"signing_key": "k"})
		require.NoError(t, err)

		assert.Equal(t, "mailgun", source.Name())
	})
}
