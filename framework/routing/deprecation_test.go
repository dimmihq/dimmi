package routing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
)

func TestDeprecated(t *testing.T) {
	t.Parallel()

	since := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	sunset := time.Date(2027, 11, 1, 0, 0, 0, 0, time.UTC)

	r := newRouter(t, http.NewKernel())
	r.API("/api", "Test API", "1.0.0", func(g *routing.Router) {
		routing.Get(g, "/items/{id}", "items.show", func(context.Context, *showInput) (*itemOutput, error) {
			return &itemOutput{}, nil
		}).Deprecated(routing.Deprecation{Since: since, Sunset: sunset, Link: "https://example.com/changes"})
		routing.Get(g, "/things/{id}", "things.show", func(context.Context, *showInput) (*itemOutput, error) {
			return &itemOutput{}, nil
		})
	})

	t.Run("tells callers of a deprecated route when it goes away", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/items/it_1", ""))

		assert.Equal(t, "@1793491200", rec.Header().Get("Deprecation"))
		assert.Equal(t, "Mon, 01 Nov 2027 00:00:00 GMT", rec.Header().Get("Sunset"))
		assert.Equal(t, `<https://example.com/changes>; rel="deprecation"; type="text/html"`, rec.Header().Get("Link"))
	})

	t.Run("leaves other routes alone", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/things/it_1", ""))

		assert.Empty(t, rec.Header().Get("Deprecation"))
		assert.Empty(t, rec.Header().Get("Sunset"))
	})

	t.Run("marks the operation deprecated in the document", func(t *testing.T) {
		t.Parallel()
		raw, err := r.OpenAPI("/api")
		require.NoError(t, err)

		var doc any
		require.NoError(t, json.Unmarshal(raw, &doc))

		assert.Equal(t, true, lookup(doc, "paths", "/items/{id}", "get", "deprecated"))
		assert.Equal(t, "2027-11-01", lookup(doc, "paths", "/items/{id}", "get", "x-sunset"))
		assert.Nil(t, lookup(doc, "paths", "/things/{id}", "get", "deprecated"))
	})
}
