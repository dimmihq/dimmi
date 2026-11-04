/*
|--------------------------------------------------------------------------
| Source Contract
|--------------------------------------------------------------------------
|
| Every inbound source must hand the application the same envelope for
| the same delivery, or the rest of the pipeline would depend on the
| provider. A source's tests call Run with a constructor returning the
| source and a way to make a delivery post the way its provider would:
| signed, and with the raw message stored where the source reads it.
|
|   func TestSource(t *testing.T) {
|       drivertest.Run(t, newHarness)
|   }
|
*/

package drivertest

import (
	"bytes"
	"crypto/rand"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
Delivery is one message as the provider received it: the raw message,
its envelope sender and recipients, and the provider's id for it.
*/
type Delivery struct {
	ID           string
	Raw          []byte
	EnvelopeFrom string
	Recipients   []string
}

/*
Post is one webhook request as the provider would send it.
*/
type Post struct {
	Header http.Header
	Query  url.Values
	Body   []byte
}

/*
Delivery returns the post as a source reads it, as many times as asked.
*/
func (p Post) Delivery() mailbox.Delivery {
	header := p.Header
	if header == nil {
		header = http.Header{}
	}

	return mailbox.Delivery{
		Header: header, Query: p.Query, Body: bytes.NewReader(p.Body), Size: int64(len(p.Body)),
	}
}

/*
Harness is a source under test.

Payload stores the raw message where the source reads it and returns the
post the provider would send. Forged returns a post for the delivery
whose signature does not check out. Junk returns a correctly signed post
that is no delivery; left nil, an unsigned one is sent. MaxBytes is the
largest message the source accepts. ExpectedID says which id a delivery
should get; left nil, the delivery's own. Keyed says the source refuses a
post it has already accepted.
*/
type Harness struct {
	Source     mailbox.Receiver
	Payload    func(t *testing.T, d Delivery) Post
	Forged     func(t *testing.T, d Delivery) Post
	Junk       func(t *testing.T) Post
	MaxBytes   int64
	ExpectedID func(d Delivery) string
	Keyed      bool
}

/*
NewHarness returns a source no other test uses.
*/
type NewHarness func(t *testing.T) Harness

/*
Run runs the contract against sources from the constructor.
*/
func Run(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("envelopes", func(t *testing.T) { t.Parallel(); runEnvelopes(t, newHarness) })
	t.Run("refusals", func(t *testing.T) { t.Parallel(); runRefusals(t, newHarness) })
	t.Run("replays", func(t *testing.T) { t.Parallel(); runReplays(t, newHarness) })
}

/*
NewDelivery returns a delivery no other test uses.
*/
func NewDelivery() Delivery {
	id := strings.ToLower(rand.Text())

	return Delivery{
		ID:           id,
		EnvelopeFrom: "bounce-" + id[:8] + "@sender.example",
		Recipients:   []string{"help@shop.example", "sales@shop.example"},
		Raw: []byte("From: Ana <ana@sender.example>\r\nTo: help@shop.example\r\nSubject: Hello\r\n" +
			"Message-ID: <" + id + "@sender.example>\r\n\r\nIs my order on its way?\r\n"),
	}
}

/*
Read returns the raw message the envelope opens.
*/
func Read(t *testing.T, env *mailbox.Envelope) []byte {
	t.Helper()

	raw, err := env.OpenRaw(t.Context())
	require.NoError(t, err)
	defer func() { require.NoError(t, raw.Close()) }()

	got, err := io.ReadAll(raw)
	require.NoError(t, err)

	return got
}

func (h Harness) expectedID(d Delivery) string {
	if h.ExpectedID != nil {
		return h.ExpectedID(d)
	}

	return d.ID
}

func runEnvelopes(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("hands over the envelope and the raw bytes as received", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		d := NewDelivery()

		env, err := h.Source.Receive(t.Context(), h.Payload(t, d).Delivery())
		require.NoError(t, err)
		require.NotNil(t, env)

		assert.Equal(t, h.Source.Name(), env.Source)
		assert.Equal(t, h.expectedID(d), env.ProviderMessageID)
		assert.Equal(t, d.EnvelopeFrom, env.EnvelopeFrom)
		assert.Equal(t, d.Recipients, env.Recipients)
		assert.Equal(t, int64(len(d.Raw)), env.SizeBytes)
		assert.False(t, env.ReceivedAt.IsZero())

		assert.True(t, bytes.Equal(d.Raw, Read(t, env)))
	})

	t.Run("gives a repeated delivery the same id", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		d := NewDelivery()
		post := h.Payload(t, d)

		first, err := h.Source.Receive(t.Context(), post.Delivery())
		require.NoError(t, err)
		again, err := h.Source.Receive(t.Context(), post.Delivery())
		require.NoError(t, err)

		assert.Equal(t, first.ProviderMessageID, again.ProviderMessageID)
	})

	t.Run("hands over a bounce with the null sender", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		d := NewDelivery()
		d.EnvelopeFrom = ""

		env, err := h.Source.Receive(t.Context(), h.Payload(t, d).Delivery())
		require.NoError(t, err)

		assert.Empty(t, env.EnvelopeFrom)
	})
}

func runRefusals(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("rejects a delivery with a bad signature", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.Source.Receive(t.Context(), h.Forged(t, NewDelivery()).Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("rejects a body that is no delivery", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		post := Post{Body: []byte("hello")}
		if h.Junk != nil {
			post = h.Junk(t)
		}

		_, err := h.Source.Receive(t.Context(), post.Delivery())

		require.ErrorIs(t, err, mailbox.ErrRejected)
	})

	t.Run("refuses a message over the size limit", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		d := NewDelivery()
		d.Raw = append(d.Raw, bytes.Repeat([]byte("x"), int(h.MaxBytes))...)

		_, err := h.Source.Receive(t.Context(), h.Payload(t, d).Delivery())

		require.ErrorIs(t, err, mailbox.ErrTooLarge)
	})
}

func runReplays(t *testing.T, newHarness NewHarness) {
	t.Helper()

	t.Run("refuses a post it accepted, not one that failed before", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		if !h.Keyed {
			t.Skip("the source keeps no replay window")
		}

		post := h.Payload(t, NewDelivery())

		failed, err := h.Source.Receive(t.Context(), post.Delivery())
		require.NoError(t, err)
		require.NotNil(t, failed.Accepted)

		retried, err := h.Source.Receive(t.Context(), post.Delivery())
		require.NoError(t, err)
		require.NoError(t, retried.Accepted(t.Context()))

		_, err = h.Source.Receive(t.Context(), post.Delivery())
		require.ErrorIs(t, err, mailbox.ErrRejected)
	})
}
