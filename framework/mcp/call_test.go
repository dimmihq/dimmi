package mcp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
seen is what the test API received.
*/
type seen struct {
	auth  string
	query string
	body  []byte
}

/*
echoAPI keeps what it receives in got and answers with fixed bodies by
path.
*/
func echoAPI(got *seen) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.body, _ = io.ReadAll(r.Body)
		got.auth, got.query = r.Header.Get("Authorization"), r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/items":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/api/items/it_1":
			w.WriteHeader(http.StatusCreated)
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte(`{"saved":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("gone"))
		}
	})
}

func TestCall(t *testing.T) {
	t.Parallel()

	from := mcp.OriginOf(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil), "Authorization")

	t.Run("make the operation's request with the caller's headers", func(t *testing.T) {
		t.Parallel()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", nil)
		r.Header.Set("Authorization", "Bearer tok")
		r.Header.Set("Cookie", "secret")
		origin := mcp.OriginOf(r, "Authorization", "Cookie", "X-Missing")
		got := &seen{}
		api := mcp.API{Handler: echoAPI(got), Prefix: "/api"}

		res, err := operations(t)["items.index"].Call(t.Context(), api, map[string]any{"status": "open"}, origin)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, res.Status)
		assert.Equal(t, "OK", res.StatusText())
		assert.Equal(t, "Bearer tok", got.auth)
		assert.Equal(t, "status=open", got.query)
		assert.Equal(t, map[string]any{"items": []any{}}, res.Decoded())
	})

	t.Run("send the body and keep the first status written", func(t *testing.T) {
		t.Parallel()
		got := &seen{}
		api := mcp.API{Handler: echoAPI(got), Prefix: "/api"}
		args := map[string]any{"item": "it_1", "name": "Box"}
		res, err := operations(t)["items.update"].Call(t.Context(), api, args, from)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, res.Status)
		assert.JSONEq(t, `{"name":"Box"}`, string(got.body))
		assert.Equal(t, map[string]any{"saved": true}, res.Decoded())
	})

	t.Run("read an answer that is not JSON as text", func(t *testing.T) {
		t.Parallel()
		api := mcp.API{Handler: echoAPI(&seen{}), Prefix: "/api"}
		res, err := operations(t)["trees.store"].Call(t.Context(), api, nil, mcp.Origin{})
		require.NoError(t, err)

		assert.Equal(t, http.StatusNotFound, res.Status)
		assert.Equal(t, "gone", res.Decoded())
	})

	t.Run("read an empty answer as nothing", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, mcp.Response{Body: []byte("  ")}.Decoded())
	})

	t.Run("refuse arguments the operation cannot be built from", func(t *testing.T) {
		t.Parallel()
		api := mcp.API{Handler: echoAPI(&seen{}), Prefix: "/api"}
		_, err := operations(t)["items.update"].Call(t.Context(), api, map[string]any{"name": "Box"}, from)

		require.ErrorContains(t, err, "missing argument")
	})
}
