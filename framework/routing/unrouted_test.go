package routing_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"
)

type unroutedKey struct{}

func TestUnrouted(t *testing.T) {
	t.Parallel()

	t.Run("clears the matched route and current request, keeping other values", func(t *testing.T) {
		t.Parallel()
		ctx := context.WithValue(t.Context(), unroutedKey{}, "kept")
		ctx = http.WithRequest(ctx, &http.Request{})

		clean := routing.Unrouted(ctx)

		_, routed := routing.CurrentRoute(clean)
		_, requested := http.RequestFrom(clean)
		assert.False(t, routed)
		assert.False(t, requested)
		assert.Equal(t, "kept", clean.Value(unroutedKey{}))
	})
}
