package routing_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/routing"
)

func TestDescribe(t *testing.T) {
	t.Parallel()

	r := routing.New(routing.Options{})
	r.API("/api/v1", "Test", "1.0.0", func(api *routing.Router) {
		routing.Get(api, "/things", "things.index", func(context.Context, *struct{}) (*struct{}, error) {
			return &struct{}{}, nil
		})
	})
	r.Describe(func(prefix string, doc map[string]any) {
		doc["x-described"] = prefix
	})

	raw, err := r.OpenAPI("/api/v1")
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	assert.Equal(t, "/api/v1", doc["x-described"])
}
