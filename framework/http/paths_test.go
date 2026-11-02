package http_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/http"
)

func TestMatchesPath(t *testing.T) {
	t.Parallel()

	t.Run("matches a path itself or by the prefix before a star", func(t *testing.T) {
		t.Parallel()

		assert.True(t, http.MatchesPath("/up", "/health", "/up"))
		assert.True(t, http.MatchesPath("/webhooks/stripe", "/webhooks/*"))
		assert.True(t, http.MatchesPath("/webhooks/", "/webhooks/*"))
		assert.False(t, http.MatchesPath("/webhooks", "/webhooks/*"))
		assert.False(t, http.MatchesPath("/upload", "/up"))
		assert.False(t, http.MatchesPath("/up"))
	})
}
