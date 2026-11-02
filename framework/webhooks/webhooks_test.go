package webhooks_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/webhooks"
)

/*
The test vector published with the Standard Webhooks libraries.
*/
const (
	vectorKey       = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	vectorID        = "msg_p5jXN8AQM9LWM0D4loKWxJek"
	vectorTimestamp = 1614265330
	vectorBody      = `{"test": 2432232314}`
	vectorSignature = "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="
)

func parse(t *testing.T, s string) webhooks.Secret {
	t.Helper()

	secret, err := webhooks.ParseSecret(s)
	require.NoError(t, err)

	return secret
}

func at(unix int64) *clock.Fake {
	return clock.NewFake(time.Unix(unix, 0))
}

func TestSign(t *testing.T) {
	t.Parallel()

	t.Run("matches the published test vector", func(t *testing.T) {
		t.Parallel()

		got := webhooks.Sign(parse(t, vectorKey), vectorID, time.Unix(vectorTimestamp, 0), []byte(vectorBody))

		assert.Equal(t, vectorSignature, got)
	})

	t.Run("matches the documented example request", func(t *testing.T) {
		t.Parallel()
		key := parse(t, "whsec_C2FVsBQIhrscChlQIMV+b5sSYspob7oD62BkjDVUOEk=")
		body := `{"id":"evt_01jbf40","object":"event","type":"webhook.test"}`

		got := webhooks.Sign(key, "evt_01jbf40", time.Unix(1793527200, 0), []byte(body))

		assert.Equal(t, "v1,ljGnIJqd/7KhrjgkZCDjChqYAfJI9ByHxCm9OHKXtwo=", got)
	})

	t.Run("puts one signature per secret in the header", func(t *testing.T) {
		t.Parallel()
		current, previous := webhooks.NewSecret(), parse(t, vectorKey)

		headers := webhooks.Headers([]webhooks.Secret{current, previous}, vectorID,
			time.Unix(vectorTimestamp, 0), []byte(vectorBody))

		entries := strings.Split(headers.Get(webhooks.HeaderSignature), " ")
		require.Len(t, entries, 2)
		assert.Equal(t, vectorSignature, entries[1])
		assert.Equal(t, vectorID, headers.Get(webhooks.HeaderID))
		assert.Equal(t, "1614265330", headers.Get(webhooks.HeaderTimestamp))
	})
}

func TestVerify(t *testing.T) {
	t.Parallel()

	signed := func(secrets ...webhooks.Secret) http.Header {
		return webhooks.Headers(secrets, vectorID, time.Unix(vectorTimestamp, 0), []byte(vectorBody))
	}
	opts := webhooks.Options{Clock: at(vectorTimestamp + 60)}

	t.Run("accepts a request signed by any current secret", func(t *testing.T) {
		t.Parallel()
		current, previous := webhooks.NewSecret(), webhooks.NewSecret()

		for _, headers := range []http.Header{signed(previous), signed(current, previous), signed(current)} {
			require.NoError(t, webhooks.Verify([]webhooks.Secret{current, previous}, headers, []byte(vectorBody), opts))
		}
	})

	t.Run("rejects a request no secret signed", func(t *testing.T) {
		t.Parallel()
		secret := webhooks.NewSecret()

		err := webhooks.Verify([]webhooks.Secret{secret}, signed(webhooks.NewSecret()), []byte(vectorBody), opts)
		require.ErrorIs(t, err, webhooks.ErrNoMatchingSignature)

		err = webhooks.Verify([]webhooks.Secret{secret}, signed(secret), []byte(vectorBody+" "), opts)
		require.ErrorIs(t, err, webhooks.ErrNoMatchingSignature)

		err = webhooks.Verify(nil, signed(secret), []byte(vectorBody), opts)
		require.ErrorIs(t, err, webhooks.ErrNoMatchingSignature)
	})

	t.Run("enforces the timestamp tolerance", func(t *testing.T) {
		t.Parallel()
		secrets := []webhooks.Secret{webhooks.NewSecret()}
		headers := signed(secrets...)
		body := []byte(vectorBody)

		err := webhooks.Verify(secrets, headers, body, webhooks.Options{Clock: at(vectorTimestamp + 301)})
		require.ErrorIs(t, err, webhooks.ErrTooOld)
		err = webhooks.Verify(secrets, headers, body, webhooks.Options{Clock: at(vectorTimestamp - 301)})
		require.ErrorIs(t, err, webhooks.ErrTooNew)

		wide := webhooks.Options{Clock: at(vectorTimestamp + 3600), Tolerance: 2 * time.Hour}
		require.NoError(t, webhooks.Verify(secrets, headers, body, wide))
	})

	t.Run("uses the system clock by default", func(t *testing.T) {
		t.Parallel()
		secrets := []webhooks.Secret{webhooks.NewSecret()}

		err := webhooks.Verify(secrets, signed(secrets...), []byte(vectorBody), webhooks.Options{})

		require.ErrorIs(t, err, webhooks.ErrTooOld)
	})

	t.Run("requires every header", func(t *testing.T) {
		t.Parallel()
		secrets := []webhooks.Secret{webhooks.NewSecret()}

		for _, name := range []string{webhooks.HeaderID, webhooks.HeaderTimestamp, webhooks.HeaderSignature} {
			headers := signed(secrets...)
			headers.Del(name)
			require.ErrorIs(t, webhooks.Verify(secrets, headers, []byte(vectorBody), opts), webhooks.ErrMissingHeaders)
		}

		headers := signed(secrets...)
		headers.Set(webhooks.HeaderTimestamp, "yesterday")
		require.ErrorIs(t, webhooks.Verify(secrets, headers, []byte(vectorBody), opts), webhooks.ErrMissingHeaders)
	})
}

func TestSecret(t *testing.T) {
	t.Parallel()

	t.Run("round trips through its encoding", func(t *testing.T) {
		t.Parallel()
		secret := webhooks.NewSecret()

		encoded := secret.Encode()

		assert.True(t, strings.HasPrefix(encoded, "whsec_"))
		assert.Len(t, encoded, len("whsec_")+44)
		assert.Equal(t, secret, parse(t, encoded))
		assert.NotEqual(t, secret, webhooks.NewSecret())
	})

	t.Run("prints redacted", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "whsec_[redacted]", webhooks.NewSecret().String())
	})

	t.Run("rejects malformed secrets", func(t *testing.T) {
		t.Parallel()

		for _, s := range []string{
			"MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw",
			"whsec_not base64!",
			"whsec_" + strings.Repeat("A", 20),
			"whsec_" + strings.Repeat("A", 88),
		} {
			_, err := webhooks.ParseSecret(s)
			require.ErrorIs(t, err, webhooks.ErrInvalidSecret, s)
		}
	})
}
