package http_test

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
)

type user struct{ ID string }

func TestContext(t *testing.T) {
	t.Parallel()

	t.Run("carries the principal of the asked type", func(t *testing.T) {
		t.Parallel()
		ctx := http.WithPrincipal(t.Context(), user{ID: "usr_1"})

		got, ok := http.Principal[user](ctx)
		assert.True(t, ok)
		assert.Equal(t, "usr_1", got.ID)

		_, ok = http.Principal[string](ctx)
		assert.False(t, ok)
	})

	t.Run("prefers the address worked out from proxies", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", stdhttp.NoBody)
		r.RemoteAddr = "10.0.0.1:5000"
		assert.Equal(t, "10.0.0.1", http.ClientIP(r))

		r.RemoteAddr = "pipe"
		assert.Equal(t, "pipe", http.ClientIP(r))

		r = r.WithContext(http.WithClientIP(r.Context(), "203.0.113.9"))
		assert.Equal(t, "203.0.113.9", http.ClientIP(r))
	})

	t.Run("carries the request id and the request", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", stdhttp.NoBody)
		ctx := http.WithRequest(http.WithRequestID(t.Context(), "req_1"), r)

		assert.Equal(t, "req_1", http.RequestID(ctx))
		assert.Empty(t, http.RequestID(t.Context()))
		got, ok := http.RequestFrom(ctx)
		assert.True(t, ok)
		assert.Same(t, r, got)
		_, ok = http.RequestFrom(t.Context())
		assert.False(t, ok)
	})
}

func TestNewCollection(t *testing.T) {
	t.Parallel()

	t.Run("writes cursors and page size under meta", func(t *testing.T) {
		t.Parallel()
		c := http.NewCollection(pagination.Page[string]{Items: []string{"a"}, NextCursor: "next", Limit: 25})
		rec := httptest.NewRecorder()
		http.WriteJSON(rec, http.StatusOK, c)

		assert.JSONEq(t, `{"data":["a"],"meta":{"next_cursor":"next","prev_cursor":null,"per_page":25}}`,
			rec.Body.String())
	})

	t.Run("keeps an empty page a list", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		http.WriteJSON(rec, http.StatusOK, http.NewCollection(pagination.Page[string]{Limit: 10}))

		assert.JSONEq(t, `{"data":[],"meta":{"next_cursor":null,"prev_cursor":null,"per_page":10}}`, rec.Body.String())
	})

	t.Run("wraps a single resource", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		http.WriteJSON(rec, http.StatusOK, http.NewResource(user{ID: "usr_1"}))

		assert.JSONEq(t, `{"data":{"ID":"usr_1"}}`, rec.Body.String())
	})
}
