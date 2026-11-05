package postmark_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/mailbox/drivers/postmark"
	"github.com/dimmihq/dimmi/framework/mailbox/drivertest"
)

func authorized(password string) http.Header {
	request := http.Request{Header: http.Header{}}
	request.SetBasicAuth("hook", password)

	return request.Header
}

func payload(t *testing.T, d drivertest.Delivery) []byte {
	t.Helper()

	return encode(t, d, string(d.Raw))
}

func payloadWithoutRaw(t *testing.T, d drivertest.Delivery) []byte {
	t.Helper()

	return encode(t, d, "")
}

/*
encode builds the post Postmark sends, without the raw message when
rawEmail is empty.
*/
func encode(t *testing.T, d drivertest.Delivery, rawEmail string) []byte {
	t.Helper()

	sender := "<" + d.EnvelopeFrom + ">"

	fields := map[string]any{
		"MessageID":         d.ID,
		"OriginalRecipient": d.Recipients[0],
		"FromFull":          map[string]string{"Email": "ana@sender.example"},
		"ToFull":            []map[string]string{{"Email": d.Recipients[1]}},
		"Headers": []map[string]string{
			{"Name": "Return-Path", "Value": sender},
			{"Name": "X-Spam-Status", "Value": "No, score=0.1"},
			{"Name": "Received-SPF", "Value": "pass (sender.example)"},
		},
	}
	if rawEmail != "" {
		fields["RawEmail"] = rawEmail
	}

	body, err := json.Marshal(fields)
	require.NoError(t, err)

	return body
}

func newSource() *postmark.Source {
	return postmark.New(clock.NewFake(time.Unix(1_800_000_000, 0)), postmark.Options{
		Username: "hook", Password: "secret", MaxWebhookBytes: 4096,
	})
}

func newHarness(t *testing.T) drivertest.Harness {
	t.Helper()

	return drivertest.Harness{
		Source:   newSource(),
		MaxBytes: 4096,
		Payload: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return drivertest.Post{Header: authorized("secret"), Body: payload(t, d)}
		},
		Forged: func(t *testing.T, d drivertest.Delivery) drivertest.Post {
			t.Helper()

			return drivertest.Post{Header: authorized("wrong"), Body: payload(t, d)}
		},
		Junk: func(t *testing.T) drivertest.Post {
			t.Helper()

			return drivertest.Post{Header: authorized("secret"), Body: []byte("hello")}
		},
	}
}

func TestSource(t *testing.T) {
	t.Parallel()

	drivertest.Run(t, newHarness)
}

func TestDelivery(t *testing.T) {
	t.Parallel()

	t.Run("reads the verdicts postmark writes", func(t *testing.T) {
		t.Parallel()
		post := drivertest.Post{
			Header: authorized("secret"), Body: payload(t, drivertest.NewDelivery()),
		}

		env, err := newSource().Receive(t.Context(), post.Delivery())
		require.NoError(t, err)

		assert.Equal(t, mailbox.Verdicts{Spam: mailbox.VerdictPass, SPF: mailbox.VerdictPass}, env.Verdicts)
	})

	t.Run("falls back to the from address without a return path", func(t *testing.T) {
		t.Parallel()
		body, err := json.Marshal(map[string]any{
			"MessageID": "m1", "RawEmail": "x", "FromFull": map[string]string{"Email": "ana@sender.example"},
		})
		require.NoError(t, err)

		env, err := newSource().Receive(t.Context(), drivertest.Post{
			Header: authorized("secret"), Body: body,
		}.Delivery())
		require.NoError(t, err)

		assert.Equal(t, "ana@sender.example", env.EnvelopeFrom)
	})

	t.Run("keeps the mail with postmark when the raw setting is off", func(t *testing.T) {
		t.Parallel()
		post := drivertest.Post{
			Header: authorized("secret"), Body: payloadWithoutRaw(t, drivertest.NewDelivery()),
		}

		_, err := newSource().Receive(t.Context(), post.Delivery())

		var misconfigured *mailbox.MisconfiguredError
		require.ErrorAs(t, err, &misconfigured)
		assert.Equal(t, postmark.RawEmailSetting, misconfigured.Setting)
		assert.Equal(t, http.StatusServiceUnavailable, mailbox.Status(newSource(), err))
		assert.Contains(t, err.Error(), "postmark")
	})

	t.Run("refuses a request with no credentials", func(t *testing.T) {
		t.Parallel()

		_, err := newSource().Receive(t.Context(), drivertest.Post{Body: []byte("{}")}.Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("tells postmark to stop retrying only a refused delivery", func(t *testing.T) {
		t.Parallel()
		s := newSource()

		assert.Equal(t, http.StatusForbidden, mailbox.Status(s, mailbox.ErrRejected))
		assert.Equal(t, http.StatusServiceUnavailable, mailbox.Status(s, errors.New("disk down")))
		assert.Equal(t, http.StatusOK, mailbox.Status(s, nil))
	})
}

func TestFactory(t *testing.T) {
	t.Parallel()

	t.Run("refuses to start without its credentials and names them", func(t *testing.T) {
		t.Parallel()

		_, err := postmark.Factory(clock.System())("postmark", config.Section{"username": "hook"})

		require.ErrorIs(t, err, mailbox.ErrMissingOption)
		assert.Contains(t, err.Error(), "mailbox.sources.postmark")
	})

	t.Run("builds a source from its options", func(t *testing.T) {
		t.Parallel()

		source, err := postmark.Factory(clock.System())("postmark", config.Section{
			"username": "hook", "password": "secret",
		})
		require.NoError(t, err)

		assert.Equal(t, "postmark", source.Name())
		receiver, ok := source.(mailbox.Receiver)
		require.True(t, ok)
		assert.Equal(t, int64(67108864), receiver.MaxWebhookBytes())
	})
}
