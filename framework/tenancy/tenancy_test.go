package tenancy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/tenancy"
)

func TestTenancy(t *testing.T) {
	t.Parallel()

	t.Run("carries the tenant through the context", func(t *testing.T) {
		t.Parallel()
		ctx := tenancy.With(t.Context(), "t1")

		id, ok := tenancy.From(ctx)
		assert.True(t, ok)
		assert.Equal(t, "t1", id)

		id, err := tenancy.Required(ctx)
		require.NoError(t, err)
		assert.Equal(t, "t1", id)
	})

	t.Run("treats an empty tenant as none", func(t *testing.T) {
		t.Parallel()

		for _, ctx := range []context.Context{t.Context(), tenancy.With(t.Context(), "")} {
			_, ok := tenancy.From(ctx)
			assert.False(t, ok)

			_, err := tenancy.Required(ctx)
			require.ErrorIs(t, err, tenancy.ErrMissing)
		}
	})
}
