package resend_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/resend"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
	"github.com/dimmihq/dimmi/framework/testkit"
	"github.com/dimmihq/dimmi/framework/webhooks"
)

const (
	apiBase = "https://api.resend.test"
	rawBase = "https://files.resend.test/raw/"
)

func now() time.Time {
	return time.Unix(1_800_000_000, 0)
}

type fixture struct {
	source *resend.Source
	fake   *httpclienttest.Fake
	secret webhooks.Secret
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	store, err := cache.Open(testkit.NewValkey(t), cache.Options{Keyspace: "cache"})
	require.NoError(t, err)
	t.Cleanup(store.Close)

	client, err := httpclient.New(httpclient.Config{AllowFakes: true})
	require.NoError(t, err)
	fake := httpclienttest.New(client).PreventStrayRequests()

	secret := webhooks.NewSecret()
	source, err := resend.New(client, clock.NewFake(now()), mailbox.NewReplays(store), resend.Options{
		WebhookSecret: secret.Encode(), APIKey: "re_test", BaseURL: apiBase,
		MaxWebhookBytes: 4096, MaxMessageBytes: 1 << 20,
	})
	require.NoError(t, err)

	return fixture{source: source, fake: fake, secret: secret}
}

func (f fixture) stubMail(t *testing.T, id string, raw []byte) {
	t.Helper()

	f.fake.Stub("GET "+apiBase+"/emails/receiving/"+id, jsonResponse(t, map[string]any{
		"raw": map[string]string{"download_url": rawBase + id},
	}))
	f.fake.Stub("GET "+rawBase+id, httpclient.Response{Status: http.StatusOK, Body: raw})
}

func jsonResponse(t *testing.T, value any) httpclient.Response {
	t.Helper()

	body, err := json.Marshal(value)
	require.NoError(t, err)

	return httpclient.Response{
		Status: http.StatusOK, Body: body, Header: http.Header{"Content-Type": {"application/json"}},
	}
}

func signed(secret webhooks.Secret, at time.Time, body []byte) drivertest.Post {
	id := "msg_" + strings.ToLower(rand.Text())

	return drivertest.Post{
		Header: http.Header{
			resend.HeaderID:        {id},
			resend.HeaderTimestamp: {strconv.FormatInt(at.Unix(), 10)},
			resend.HeaderSignature: {webhooks.Sign(secret, id, at, body)},
		},
		Body: body,
	}
}

func eventBody(t *testing.T, d drivertest.Delivery) []byte {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"type": "email.received",
		"data": map[string]any{
			"email_id": d.ID, "from": d.EnvelopeFrom, "to": d.Recipients[:1], "cc": d.Recipients[1:], "bcc": []string{},
		},
	})
	require.NoError(t, err)

	return body
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	f := newFixture(t)

	return drivertest.Harness{
		Source:   f.source,
		MaxBytes: 1 << 20,
		Keyed:    true,
		Payload: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			f.stubMail(t, d.ID, d.Raw)

			return signed(f.secret, now(), eventBody(t, d))
		},
		Forged: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return signed(webhooks.NewSecret(), now(), eventBody(t, d))
		},
		Junk: func(t *testing.T) drivertest.Post {
			t.Helper()

			return signed(f.secret, now(), []byte("hello"))
		},
	}
}

func TestSource(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestDelivery(t *testing.T) {
	t.Parallel()

	t.Run("sends the api key to fetch the raw message", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		d := drivertest.NewDelivery()
		f.stubMail(t, d.ID, d.Raw)

		_, err := f.source.Receive(t.Context(), signed(f.secret, now(), eventBody(t, d)).Delivery())
		require.NoError(t, err)

		f.fake.AssertSent(t, func(req httpclienttest.SentRequest) bool {
			return req.Header.Get("Authorization") == "Bearer re_test"
		})
	})

	t.Run("ignores other events", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		env, err := f.source.Receive(t.Context(),
			signed(f.secret, now(), []byte(`{"type":"email.sent"}`)).Delivery())

		require.NoError(t, err)
		assert.Nil(t, env)
	})

	t.Run("fails for a retry while resend has no raw message", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		d := drivertest.NewDelivery()
		f.fake.Stub("GET "+apiBase+"/emails/receiving/"+d.ID, jsonResponse(t, map[string]any{"raw": nil}))

		_, err := f.source.Receive(t.Context(), signed(f.secret, now(), eventBody(t, d)).Delivery())

		require.ErrorIs(t, err, resend.ErrRawPending)
		assert.Equal(t, http.StatusServiceUnavailable, mailbox.Status(f.source, err))
	})

	t.Run("fails for a retry when the link stays expired", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		d := drivertest.NewDelivery()
		f.fake.Stub("GET "+apiBase+"/emails/receiving/"+d.ID, jsonResponse(t, map[string]any{
			"raw": map[string]string{"download_url": rawBase + d.ID},
		}))
		f.fake.Stub("GET "+rawBase+d.ID, httpclient.Response{Status: http.StatusForbidden})

		_, err := f.source.Receive(t.Context(), signed(f.secret, now(), eventBody(t, d)).Delivery())

		require.Error(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, mailbox.Status(f.source, err))
	})

	t.Run("fails for a retry when the api is down", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		d := drivertest.NewDelivery()
		f.fake.Stub("GET "+apiBase+"/emails/receiving/"+d.ID, httpclient.Response{Status: http.StatusBadGateway})

		_, err := f.source.Receive(t.Context(), signed(f.secret, now(), eventBody(t, d)).Delivery())

		require.Error(t, err)
		assert.NotErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("fetches its raw message itself", func(t *testing.T) {
		t.Parallel()

		assert.True(t, newFixture(t).source.FetchesRaw())
	})

	t.Run("refuses a post over the limit", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		_, err := f.source.Receive(t.Context(), signed(f.secret, now(), make([]byte, 5000)).Delivery())

		require.ErrorIs(t, err, mailbox.ErrTooLarge)
	})

	t.Run("refuses a signature older than five minutes", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		_, err := f.source.Receive(t.Context(),
			signed(f.secret, now().Add(-6*time.Minute), eventBody(t, drivertest.NewDelivery())).Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})
}

func TestBackfill(t *testing.T) {
	t.Parallel()

	list := func(t *testing.T, f fixture, ids map[string]string, more bool) {
		t.Helper()

		rows := []map[string]any{}
		for id, created := range ids {
			rows = append(rows, map[string]any{
				"id": id, "created_at": created, "from": "ana@sender.example", "to": []string{"help@shop.example"},
			})
		}

		f.fake.Stub("GET "+apiBase+"/emails/receiving?limit=100", jsonResponse(t, map[string]any{
			"data": rows, "has_more": more,
		}))
	}

	t.Run("hands over mail since the given time", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.stubMail(t, "m1", []byte("raw one"))
		list(t, f, map[string]string{"m1": "2027-01-15 10:00:00.5+00"}, false)

		var got []mailbox.Received

		err := f.source.Backfill(t.Context(), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			func(_ context.Context, r mailbox.Received) error { got = append(got, r); return nil })
		require.NoError(t, err)

		require.Len(t, got, 1)
		assert.Equal(t, "m1", got[0].ID)
		assert.Equal(t, []byte("raw one"), drivertest.Read(t, got[0].Envelope))
	})

	t.Run("stops at mail older than the given time", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		list(t, f, map[string]string{"m1": "2020-01-15T10:00:00Z"}, true)

		called := false
		err := f.source.Backfill(t.Context(), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			func(context.Context, mailbox.Received) error { called = true; return nil })

		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("passes on the handler's error", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.stubMail(t, "m1", []byte("raw one"))
		list(t, f, map[string]string{"m1": "2027-01-15T10:00:00Z"}, false)
		failure := errors.New("full")

		err := f.source.Backfill(t.Context(), time.Time{},
			func(context.Context, mailbox.Received) error { return failure })

		require.ErrorIs(t, err, failure)
	})

	t.Run("walks every page", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.stubMail(t, "m1", []byte("raw one"))
		f.stubMail(t, "m2", []byte("raw two"))
		list(t, f, map[string]string{"m1": "2027-01-15T10:00:00Z"}, true)
		f.fake.Stub("GET "+apiBase+"/emails/receiving?after=m1&limit=100", jsonResponse(t, map[string]any{
			"data": []map[string]any{{"id": "m2", "created_at": "2027-01-14T10:00:00Z"}}, "has_more": false,
		}))

		var got []string

		err := f.source.Backfill(t.Context(), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			func(_ context.Context, r mailbox.Received) error { got = append(got, r.ID); return nil })

		require.NoError(t, err)
		assert.Equal(t, []string{"m1", "m2"}, got)
	})

	t.Run("treats a time it cannot read as older than any", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		list(t, f, map[string]string{"m1": "yesterday"}, false)

		called := false
		err := f.source.Backfill(t.Context(), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			func(context.Context, mailbox.Received) error { called = true; return nil })

		require.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("reports a mail whose raw message cannot be fetched", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		list(t, f, map[string]string{"m1": "2027-01-15T10:00:00Z"}, false)

		err := f.source.Backfill(t.Context(), time.Time{},
			func(context.Context, mailbox.Received) error { return nil })

		require.ErrorIs(t, err, httpclienttest.ErrStrayRequest)
	})

	t.Run("reports a list or a mail it cannot decode", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.fake.Stub("GET "+apiBase+"/emails/receiving?limit=100", httpclient.Response{
			Status: http.StatusOK, Body: []byte("nope"),
		})

		err := f.source.Backfill(t.Context(), time.Time{},
			func(context.Context, mailbox.Received) error { return nil })
		require.Error(t, err)

		g := newFixture(t)
		list(t, g, map[string]string{"m1": "2027-01-15T10:00:00Z"}, false)
		g.fake.Stub("GET "+apiBase+"/emails/receiving/m1",
			httpclient.Response{Status: http.StatusOK, Body: []byte("nope")})

		err = g.source.Backfill(t.Context(), time.Time{},
			func(context.Context, mailbox.Received) error { return nil })
		require.Error(t, err)
	})

	t.Run("reports a list it cannot reach", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)

		err := f.source.Backfill(t.Context(), time.Time{},
			func(context.Context, mailbox.Received) error { return nil })

		require.ErrorIs(t, err, httpclienttest.ErrStrayRequest)
	})

	t.Run("reports a list resend refuses", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t)
		f.fake.Stub("GET "+apiBase+"/emails/receiving?limit=100", httpclient.Response{Status: http.StatusBadGateway})

		err := f.source.Backfill(t.Context(), time.Time{},
			func(context.Context, mailbox.Received) error { return nil })

		require.Error(t, err)
	})
}

func TestFactory(t *testing.T) {
	t.Parallel()

	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("0123456789012345678901234567"))

	t.Run("refuses to start without its secret and key and names them", func(t *testing.T) {
		t.Parallel()

		_, err := resend.Factory(nil, clock.System(), nil)("resend", config.Section{"webhook_secret": secret})

		require.ErrorIs(t, err, mailbox.ErrMissingOption)
		assert.Contains(t, err.Error(), "mailbox.sources.resend")
	})

	t.Run("refuses a malformed secret", func(t *testing.T) {
		t.Parallel()

		_, err := resend.Factory(nil, clock.System(), nil)("resend", config.Section{
			"webhook_secret": "nope", "api_key": "k",
		})

		require.ErrorIs(t, err, webhooks.ErrInvalidSecret)
	})

	t.Run("builds a source from its options", func(t *testing.T) {
		t.Parallel()

		source, err := resend.Factory(nil, clock.System(), nil)("resend", config.Section{
			"webhook_secret": secret, "api_key": "k",
		})
		require.NoError(t, err)

		assert.Equal(t, "resend", source.Name())
		receiver, ok := source.(mailbox.Receiver)
		require.True(t, ok)
		assert.Equal(t, int64(1048576), receiver.MaxWebhookBytes())
	})
}
