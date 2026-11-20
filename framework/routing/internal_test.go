package routing_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
)

func TestInternal(t *testing.T) {
	t.Parallel()

	r := newRouter(t, http.NewKernel())
	r.API("/api", "Test API", "1.0.0", func(g *routing.Router) {
		routing.Get(g, "/items/{id}", "items.show", func(context.Context, *showInput) (*itemOutput, error) {
			return &itemOutput{}, nil
		}).Internal()
		routing.Get(g, "/things/{id}", "things.show", func(context.Context, *showInput) (*itemOutput, error) {
			return &itemOutput{}, nil
		})
		g.Group("", func(own *routing.Router) {
			own.Internal()
			routing.Get(own, "/drafts/{id}", "drafts.show", func(context.Context, *showInput) (*itemOutput, error) {
				return &itemOutput{}, nil
			})
		})
	})

	raw, err := r.OpenAPI("/api")
	require.NoError(t, err)

	var doc any
	require.NoError(t, json.Unmarshal(raw, &doc))

	t.Run("marks a route marked internal", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, true, lookup(doc, "paths", "/items/{id}", "get", "x-internal"))
	})

	t.Run("marks every route of an internal group", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, true, lookup(doc, "paths", "/drafts/{id}", "get", "x-internal"))
	})

	t.Run("leaves public routes unmarked", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, lookup(doc, "paths", "/things/{id}", "get", "x-internal"))
	})

	t.Run("still serves internal routes", func(t *testing.T) {
		t.Parallel()
		rec := serve(t, r, request(t, http.MethodGet, "/api/items/it_1", ""))
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
