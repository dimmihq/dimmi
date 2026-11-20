package entitlements_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/support/entitlements"
)

func TestSelfHostWebhookEndpoints(t *testing.T) {
	t.Parallel()

	limit := func(t *testing.T, c entitlements.Catalog) int64 {
		t.Helper()

		n, err := entitlements.Driver{Catalog: c}.Limit(t.Context(), ids.Nil(), entitlements.KeyWebhookEndpoints)
		require.NoError(t, err)

		return n
	}

	t.Run("the configured number is the cap", func(t *testing.T) {
		t.Parallel()

		catalog := entitlements.SelfHostWith(entitlements.SelfHostLimits{WebhookEndpoints: 50})
		assert.EqualValues(t, 50, limit(t, catalog))
	})

	t.Run("zero means no limit", func(t *testing.T) {
		t.Parallel()

		catalog := entitlements.SelfHostWith(entitlements.SelfHostLimits{})
		assert.EqualValues(t, entitlements.Unlimited, limit(t, catalog))
	})

	t.Run("the widget-only catalog keeps the default of 20", func(t *testing.T) {
		t.Parallel()

		assert.EqualValues(t, 20, limit(t, entitlements.SelfHost(0)))
	})

	t.Run("the cloud keeps its own cap", func(t *testing.T) {
		t.Parallel()

		assert.EqualValues(t, 20, limit(t, entitlements.Cloud()))
	})
}
