package routing_test

import (
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
)

func newURL(t *testing.T, key []byte) (*routing.URL, *clock.Fake) {
	t.Helper()

	r := routing.New(routing.Options{})
	r.Group("/files", func(g *routing.Router) {
		g.Handle(http.MethodGet, "/{file}/{name:[a-z.]+}", "files.show", nothing())
	})
	r.Handle(http.MethodGet, "/unsubscriptions", "unsubscriptions.index", nothing())

	clk := clock.NewFake(time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC))

	return routing.NewURL(r, "https://app.example.com/", key, clk), clk
}

func verify(t *testing.T, u *routing.URL, link string) bool {
	t.Helper()

	return u.HasValidSignature(httptest.NewRequestWithContext(t.Context(), http.MethodGet, link, stdhttp.NoBody))
}

func TestURL(t *testing.T) {
	t.Parallel()

	t.Run("fills placeholders and puts the rest in the query", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, nil)

		link, err := u.Route("files.show", map[string]string{"file": "fil 1", "name": "a.pdf", "inline": "1"})
		require.NoError(t, err)
		assert.Equal(t, "https://app.example.com/files/fil%201/a.pdf?inline=1", link)
	})

	t.Run("refuses unknown names", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, nil)

		_, err := u.Route("files.index", nil)
		require.ErrorIs(t, err, routing.ErrUnknownName)
	})

	t.Run("refuses missing parameters", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, nil)

		_, err := u.Route("files.show", map[string]string{"file": "fil_1"})
		require.ErrorIs(t, err, routing.ErrMissingParameter)
	})

	t.Run("panics on a broken fixed link", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, nil)

		assert.Equal(t, "https://app.example.com/unsubscriptions", u.MustRoute("unsubscriptions.index", nil))
		assert.Panics(t, func() { u.MustRoute("nowhere", nil) })
	})
}

func TestSignedRoute(t *testing.T) {
	t.Parallel()

	key := []byte("secret-key")

	t.Run("verifies its own links", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, key)

		link, err := u.SignedRoute("files.show", map[string]string{"file": "fil_1", "name": "a.pdf"}, time.Hour)
		require.NoError(t, err)
		assert.Contains(t, link, "expires=")
		assert.True(t, verify(t, u, link))
	})

	t.Run("verifies links without an expiry", func(t *testing.T) {
		t.Parallel()
		u, clk := newURL(t, key)

		link, err := u.SignedRoute("unsubscriptions.index", map[string]string{"user": "usr_1"}, 0)
		require.NoError(t, err)
		clk.Travel(24 * 365 * time.Hour)

		assert.NotContains(t, link, "expires=")
		assert.True(t, verify(t, u, link))
	})

	t.Run("refuses changed links", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, key)
		link, err := u.SignedRoute("unsubscriptions.index", map[string]string{"user": "usr_1"}, time.Hour)
		require.NoError(t, err)

		assert.False(t, verify(t, u, strings.Replace(link, "usr_1", "usr_2", 1)))
		assert.False(t, verify(t, u, strings.Replace(link, "signature=", "signature=00", 1)))
		assert.False(t, verify(t, u, strings.Replace(link, "signature=", "signature=zz", 1)))
	})

	t.Run("refuses expired links", func(t *testing.T) {
		t.Parallel()
		u, clk := newURL(t, key)
		link, err := u.SignedRoute("unsubscriptions.index", nil, time.Minute)
		require.NoError(t, err)

		clk.Travel(time.Minute)
		assert.True(t, verify(t, u, link))
		clk.Travel(time.Second)
		assert.False(t, verify(t, u, link))
	})

	t.Run("refuses an expiry that is not a time", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, key)
		link, err := u.SignedRoute("unsubscriptions.index", map[string]string{"expires": "soon"}, 0)
		require.NoError(t, err)

		assert.False(t, verify(t, u, link))
	})

	t.Run("refuses unsigned links and links signed with another key", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, key)
		other, _ := newURL(t, []byte("other-key"))
		link, err := other.SignedRoute("unsubscriptions.index", nil, time.Hour)
		require.NoError(t, err)

		assert.False(t, verify(t, u, link))
		assert.False(t, verify(t, u, "https://app.example.com/unsubscriptions"))
	})

	t.Run("needs a key", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, nil)

		_, err := u.SignedRoute("unsubscriptions.index", nil, time.Hour)
		require.ErrorIs(t, err, routing.ErrNoSigningKey)
		assert.False(t, verify(t, u, "/unsubscriptions?signature=00"))
	})

	t.Run("refuses unknown names", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, key)

		_, err := u.SignedRoute("nowhere", nil, time.Hour)
		require.ErrorIs(t, err, routing.ErrUnknownName)
	})

	t.Run("signs the query in a stable order", func(t *testing.T) {
		t.Parallel()
		u, _ := newURL(t, key)
		link, err := u.SignedRoute("unsubscriptions.index", map[string]string{"b": "2", "a": "1"}, 0)
		require.NoError(t, err)

		parsed, err := url.Parse(link)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(parsed.RawQuery, "a=1&b=2&signature="))
	})
}
