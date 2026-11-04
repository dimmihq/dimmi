package cloudflare_test

import (
	"net/http"
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
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/cloudflare"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
	"github.com/dimmihq/dimmi/framework/testkit"
)

const secret = "shared-secret"

func now() time.Time {
	return time.Unix(1_800_000_000, 0)
}

func newSource(t *testing.T) *cloudflare.Source {
	t.Helper()

	store, err := cache.Open(testkit.NewValkey(t), cache.Options{Keyspace: "cache"})
	require.NoError(t, err)
	t.Cleanup(store.Close)

	return cloudflare.New(clock.NewFake(now()), mailbox.NewReplays(store), cloudflare.Options{
		Secret: secret, MaxWebhookBytes: 4096,
	})
}

func post(key string, at time.Time, d drivertest.Delivery) drivertest.Post {
	timestamp := strconv.FormatInt(at.Unix(), 10)

	return drivertest.Post{
		Header: http.Header{
			cloudflare.HeaderTimestamp:    {timestamp},
			cloudflare.HeaderSignature:    {cloudflare.Sign(key, timestamp, d.Raw)},
			cloudflare.HeaderEnvelopeFrom: {d.EnvelopeFrom},
			cloudflare.HeaderEnvelopeTo:   {strings.Join(d.Recipients, ",")},
		},
		Body: d.Raw,
	}
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	return drivertest.Harness{
		Source:   newSource(t),
		MaxBytes: 4096,
		Keyed:    true,
		ExpectedID: func(d drivertest.Delivery) string {
			return hashing.SHA256Hex(d.Raw)[:32]
		},
		Payload: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return post(secret, now(), d)
		},
		Forged: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return post("other", now(), d)
		},
	}
}

func TestSource(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestDelivery(t *testing.T) {
	t.Parallel()

	withHeader := func(header string) drivertest.Delivery {
		d := drivertest.NewDelivery()
		d.Raw = append([]byte(header), d.Raw...)

		return d
	}

	t.Run("believes the top header when cloudflare wrote it", func(t *testing.T) {
		t.Parallel()
		d := withHeader("Authentication-Results: mx.cloudflare.net; dkim=pass header.d=sender.example; " +
			"spf=fail smtp.mailfrom=sender.example; dmarc=pass header.from=sender.example\r\n")

		env, err := newSource(t).Receive(t.Context(), post(secret, now(), d).Delivery())
		require.NoError(t, err)

		assert.Equal(t, mailbox.Verdicts{
			SPF: mailbox.VerdictFail, DKIM: mailbox.VerdictPass, DMARC: mailbox.VerdictPass,
		}, env.Verdicts)
	})

	t.Run("believes nothing when another host wrote the top header", func(t *testing.T) {
		t.Parallel()
		d := withHeader("Authentication-Results: evil.example; dkim=pass\r\n" +
			"Authentication-Results: mx.cloudflare.net; dkim=fail\r\n")

		env, err := newSource(t).Receive(t.Context(), post(secret, now(), d).Delivery())
		require.NoError(t, err)

		assert.Equal(t, mailbox.Verdicts{}, env.Verdicts)
	})

	t.Run("believes nothing without the header", func(t *testing.T) {
		t.Parallel()

		env, err := newSource(t).Receive(t.Context(), post(secret, now(), drivertest.NewDelivery()).Delivery())
		require.NoError(t, err)

		assert.Equal(t, mailbox.Verdicts{}, env.Verdicts)
	})

	t.Run("refuses a signature older than five minutes", func(t *testing.T) {
		t.Parallel()

		_, err := newSource(t).Receive(t.Context(),
			post(secret, now().Add(-6*time.Minute), drivertest.NewDelivery()).Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})
}

func TestFactory(t *testing.T) {
	t.Parallel()

	t.Run("refuses to start without its secret and names it", func(t *testing.T) {
		t.Parallel()

		_, err := cloudflare.Factory(clock.System(), nil)("cloudflare", config.Section{})

		require.ErrorIs(t, err, mailbox.ErrMissingOption)
		assert.Contains(t, err.Error(), "mailbox.sources.cloudflare.secret")
	})

	t.Run("builds a source from its options", func(t *testing.T) {
		t.Parallel()

		source, err := cloudflare.Factory(clock.System(), nil)("cloudflare", config.Section{"secret": "s"})
		require.NoError(t, err)

		assert.Equal(t, "cloudflare", source.Name())
	})
}
