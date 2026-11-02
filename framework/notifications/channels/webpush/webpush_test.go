package webpush_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
)

func TestSend(t *testing.T) {
	t.Parallel()

	t.Run("encrypts the payload for the subscription and signs the request", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)
		f.fake.Stub("POST https://push.example.com/*", httpclient.Response{Status: http.StatusCreated})
		phone := newDevice(t, "https://push.example.com/send/abc")

		require.NoError(t, f.manager.Send(t.Context(), alert{Text: `{"title":"Hi"}`}, member{route: phone.sub}))

		f.fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			c, key := claims(t, req.Header.Get("Authorization"))

			return req.URL.String() == phone.sub.Endpoint &&
				string(phone.decrypt(t, req.Body)) == `{"title":"Hi"}` &&
				req.Header.Get("Content-Encoding") == "aes128gcm" &&
				req.Header.Get("TTL") == "14400" &&
				req.Header.Get("Urgency") == "high" &&
				req.Header.Get("Topic") == "c1" &&
				key == f.cfg.VAPIDPublicKey &&
				c["sub"] == "mailto:ops@example.com" &&
				c["aud"] == "https://push.example.com"
		})
	})

	t.Run("pushes to every subscription with the message's own time to live", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "https://example.com/about", nil)
		f.fake.Stub("POST https://push.example.com/*", httpclient.Response{Status: http.StatusCreated})
		phone, laptop := newDevice(t, "https://push.example.com/phone"), newDevice(t, "https://push.example.com/laptop")

		require.NoError(t, f.manager.Send(t.Context(), alert{Text: "hi", TTL: time.Minute},
			member{route: []webpush.Subscription{phone.sub, laptop.sub}}))

		for _, d := range []device{phone, laptop} {
			f.fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
				c, _ := claims(t, req.Header.Get("Authorization"))

				return req.URL.String() == d.sub.Endpoint && req.Header.Get("TTL") == "60" &&
					c["sub"] == "https://example.com/about"
			})
		}
	})

	t.Run("reports a gone subscription and still pushes to the rest", func(t *testing.T) {
		t.Parallel()
		gone := &goneList{}
		f := newFixture(t, "mailto:ops@example.com", gone.report)
		f.fake.Stub("POST https://push.example.com/old", httpclient.Response{Status: http.StatusGone})
		f.fake.Stub("POST https://push.example.com/*", httpclient.Response{Status: http.StatusCreated})
		old, current := newDevice(t, "https://push.example.com/old"), newDevice(t, "https://push.example.com/new")

		require.NoError(t, f.manager.Send(t.Context(), alert{Text: "hi"},
			member{route: []webpush.Subscription{old.sub, current.sub}}))

		assert.Equal(t, []webpush.Subscription{old.sub}, gone.subs)
		f.fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.URL.String() == current.sub.Endpoint
		})
	})

	t.Run("returns the failure to report a gone subscription", func(t *testing.T) {
		t.Parallel()
		gone := &goneList{err: errors.New("database unavailable")}
		f := newFixture(t, "mailto:ops@example.com", gone.report)
		f.fake.Stub("POST https://push.example.com/*", httpclient.Response{Status: http.StatusNotFound})

		err := f.manager.Send(t.Context(), alert{}, member{route: newDevice(t, "https://push.example.com/x").sub})

		require.ErrorContains(t, err, "push alert: database unavailable")
	})

	t.Run("returns a gone subscription as a permanent error without a gone func", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)
		f.fake.Stub("POST https://push.example.com/*", httpclient.Response{Status: http.StatusGone, Body: []byte("expired\n")})

		err := f.manager.Send(t.Context(), alert{}, member{route: newDevice(t, "https://push.example.com/x").sub})

		require.ErrorIs(t, err, webpush.ErrGone)
		require.ErrorIs(t, err, notifications.ErrPermanent)
		require.ErrorContains(t, err, "webpush: push.example.com answered 410 expired")
		require.NotContains(t, err.Error(), "/x")
	})

	t.Run("skips a recipient without subscriptions", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)
		f.fake.PreventStrayRequests()

		require.NoError(t, f.manager.Send(t.Context(), alert{}, member{}, member{route: []webpush.Subscription{}}))
	})

	t.Run("refuses what it cannot push", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)
		f.fake.PreventStrayRequests()
		to := member{route: newDevice(t, "https://push.example.com/x").sub}

		require.ErrorIs(t, f.manager.Send(t.Context(), alert{}, member{route: "x"}), notifications.ErrInvalidRoute)
		require.ErrorIs(t, f.manager.Send(t.Context(), silent{}, to), notifications.ErrUnsupported)
		require.ErrorContains(t, f.manager.Send(t.Context(), alert{Fail: true}, to),
			"render alert for push: payload too big to trim")
	})
}

func TestPush(t *testing.T) {
	t.Parallel()

	t.Run("keeps how a refusal can be handled", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			status    int
			gone      bool
			permanent bool
		}{
			{http.StatusNotFound, true, true},
			{http.StatusGone, true, true},
			{http.StatusBadRequest, false, true},
			{http.StatusForbidden, false, true},
			{http.StatusRequestEntityTooLarge, false, true},
			{http.StatusRequestTimeout, false, false},
			{http.StatusTooManyRequests, false, false},
			{http.StatusServiceUnavailable, false, false},
		}

		for _, tc := range cases {
			f := newFixture(t, "mailto:ops@example.com", nil)
			f.fake.Stub("POST https://push.example.com/*", httpclient.Response{Status: tc.status})

			err := f.channel(t).Push(t.Context(), newDevice(t, "https://push.example.com/x").sub, webpush.Message{})

			statusErr, ok := errors.AsType[*webpush.StatusError](err)
			require.True(t, ok, tc.status)
			assert.Equal(t, tc.status, statusErr.Status)
			assert.Equal(t, tc.gone, errors.Is(err, webpush.ErrGone), tc.status)
			assert.Equal(t, tc.permanent, errors.Is(err, notifications.ErrPermanent), tc.status)
			assert.False(t, errors.Is(err, context.Canceled), tc.status)
		}
	})

	t.Run("names the push service without the endpoint's secret path", func(t *testing.T) {
		t.Parallel()

		err := &webpush.StatusError{Endpoint: "https://push.example.com/secret", Status: 500}
		assert.Equal(t, "webpush: push.example.com answered 500 ", err.Error())

		err = &webpush.StatusError{Endpoint: "::", Status: 500}
		assert.Equal(t, "webpush: push service answered 500 ", err.Error())
	})

	t.Run("reads how long a rate limit asks to wait", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)
		date := f.clock.Now().Add(2 * time.Minute).Format(http.TimeFormat)

		headers := map[string]time.Duration{"30": 30 * time.Second, date: 2 * time.Minute, "soon": 0, "": 0}
		for header, want := range headers {
			path := "/" + base64.RawURLEncoding.EncodeToString([]byte(header))
			f.fake.Stub("POST https://push.example.com"+path, httpclient.Response{
				Status: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {header}},
			})

			err := f.channel(t).Push(t.Context(), newDevice(t, "https://push.example.com"+path).sub, webpush.Message{})

			statusErr, ok := errors.AsType[*webpush.StatusError](err)
			require.True(t, ok)
			assert.Equal(t, want, statusErr.RetryAfter, header)
		}
	})

	t.Run("refuses a malformed subscription without a request", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)
		f.fake.PreventStrayRequests()
		good := newDevice(t, "https://push.example.com/x").sub

		bad := []webpush.Subscription{
			{Endpoint: "http://push.example.com/x", Keys: good.Keys},
			{Endpoint: "https://push.example.com/x", Keys: webpush.Keys{Auth: "short", P256dh: good.Keys.P256dh}},
			{Endpoint: "https://push.example.com/x", Keys: webpush.Keys{Auth: good.Keys.Auth, P256dh: "!!"}},
			{Endpoint: "https://push.example.com/x", Keys: webpush.Keys{Auth: good.Keys.Auth, P256dh: good.Keys.Auth}},
		}

		for _, sub := range bad {
			err := f.channel(t).Push(t.Context(), sub, webpush.Message{})

			require.ErrorIs(t, err, webpush.ErrInvalidSubscription)
			require.ErrorIs(t, err, notifications.ErrPermanent)
		}
	})

	t.Run("accepts keys in the standard base64 alphabet", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)
		f.fake.Stub("POST https://push.example.com/*", httpclient.Response{Status: http.StatusCreated})
		d := newDevice(t, "https://push.example.com/x")
		d.sub.Keys.Auth = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xff}, 16))

		require.NoError(t, f.channel(t).Push(t.Context(), d.sub, webpush.Message{Payload: []byte("hi")}))
	})

	t.Run("refuses an endpoint at a private address for good", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)

		err := f.channel(t).Push(t.Context(), newDevice(t, "https://10.0.0.1/x").sub, webpush.Message{})

		require.ErrorIs(t, err, httpclient.ErrBlockedHost)
		require.ErrorIs(t, err, notifications.ErrPermanent)
	})

	t.Run("leaves a request that never reached the push service retryable", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, "mailto:ops@example.com", nil)
		f.fake.PreventStrayRequests()

		err := f.channel(t).Push(t.Context(), newDevice(t, "https://push.example.com/x").sub, webpush.Message{})

		require.ErrorIs(t, err, httpclienttest.ErrStrayRequest)
		require.NotErrorIs(t, err, notifications.ErrPermanent)
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	private, public, err := webpush.GenerateKeys()
	require.NoError(t, err)

	client, err := httpclient.New(httpclient.Config{})
	require.NoError(t, err)

	t.Run("refuses missing or malformed vapid settings", func(t *testing.T) {
		t.Parallel()

		configs := []webpush.Config{
			{VAPIDPublicKey: public, VAPIDPrivateKey: "", Subject: "mailto:ops@example.com"},
			{VAPIDPublicKey: private, VAPIDPrivateKey: private, Subject: "mailto:ops@example.com"},
			{VAPIDPublicKey: public, VAPIDPrivateKey: private, Subject: "ops@example.com"},
		}

		for _, cfg := range configs {
			_, err := webpush.New(cfg, client, clock.System(), nil)

			require.ErrorIs(t, err, webpush.ErrInvalidConfig)
		}
	})

	t.Run("reads its settings from the channel's configuration", func(t *testing.T) {
		t.Parallel()
		factory := webpush.Factory(client, clock.System(), nil)

		_, err := factory("webpush", config.Section{
			"vapid_public_key": public, "vapid_private_key": private, "subject": "mailto:ops@example.com",
		})
		require.NoError(t, err)

		_, err = factory("webpush", config.Section{"ttl": "forever"})
		require.ErrorContains(t, err, "webpush")

		_, err = factory("webpush", config.Section{})
		require.ErrorIs(t, err, webpush.ErrInvalidConfig)
	})
}
