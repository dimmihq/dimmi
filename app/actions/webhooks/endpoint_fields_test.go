package webhooks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/exceptions"
)

func TestNormalizeEventTypes(t *testing.T) {
	t.Parallel()

	t.Run("refuses an unknown type", func(t *testing.T) {
		t.Parallel()

		_, err := normalizeEventTypes([]string{"nothing.happened"})

		var f *fault.Error
		require.ErrorAs(t, err, &f)
		assert.Equal(t, "unknown_event_type", f.Code())
		assert.Equal(t, exceptions.ErrUnknownWebhookEventType.Message(), f.Message())
		assert.Equal(t, "nothing.happened", f.Meta()["event_type"])
	})

	t.Run("refuses too many entries under the same code with its own message", func(t *testing.T) {
		t.Parallel()

		// The catalog has fewer than 100 distinct entries today, so the
		// cap is checked on the fault it raises rather than through input.
		f := exceptions.ErrTooManyWebhookEventTypes.With("max", maxEventTypes)

		assert.Equal(t, exceptions.ErrUnknownWebhookEventType.Code(), f.Code())
		assert.Equal(t, "event_types", f.Field())
		assert.NotEqual(t, exceptions.ErrUnknownWebhookEventType.Message(), f.Message())
		assert.Contains(t, f.Message(), "at most 100")
	})
}

func TestRotationOverlapBlamesTheRequestField(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "overlap_seconds", exceptions.ErrWebhookRotationOverlap.Field())
}
