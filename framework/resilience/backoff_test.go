package resilience_test

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/resilience"
)

func TestBackoff(t *testing.T) {
	t.Parallel()

	t.Run("grows each wait by the multiplier up to the ceiling", func(t *testing.T) {
		t.Parallel()
		policy := resilience.Policy{Initial: time.Second, Max: 5 * time.Second, Multiplier: 2, Retries: 5}

		waits := slices.Collect(resilience.Backoff(policy))

		want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
		assert.Equal(t, want, waits)
	})

	t.Run("starts over on every range", func(t *testing.T) {
		t.Parallel()
		seq := resilience.Backoff(resilience.Policy{Initial: time.Second, Multiplier: 2, Retries: 2})

		assert.Equal(t, slices.Collect(seq), slices.Collect(seq))
	})

	t.Run("uses the defaults for zero fields", func(t *testing.T) {
		t.Parallel()

		waits := slices.Collect(resilience.Backoff(resilience.Policy{Retries: 2}))

		assert.Equal(t, []time.Duration{resilience.DefaultInitial, 750 * time.Millisecond}, waits)
	})

	t.Run("spreads waits by the jitter", func(t *testing.T) {
		t.Parallel()

		for wait := range resilience.Backoff(resilience.Policy{Initial: time.Second, Jitter: 0.5, Retries: 20}) {
			assert.GreaterOrEqual(t, wait, 500*time.Millisecond)
			assert.LessOrEqual(t, wait, resilience.DefaultMax+resilience.DefaultMax/2)
		}
	})

	t.Run("keeps going without a retry cap until the caller stops", func(t *testing.T) {
		t.Parallel()
		count := 0

		for range resilience.Backoff(resilience.Policy{}) {
			count++
			if count == 100 {
				break
			}
		}

		assert.Equal(t, 100, count)
	})
}
