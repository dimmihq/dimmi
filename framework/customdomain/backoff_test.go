package customdomain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/customdomain"
)

func TestSchedule(t *testing.T) {
	t.Parallel()

	t.Run("waits longer after each early failure", func(t *testing.T) {
		t.Parallel()
		schedule := customdomain.DefaultSchedule()

		var waits []time.Duration

		for failures := 1; failures <= 6; failures++ {
			wait, ok := schedule.Next(failures, time.Hour)
			assert.True(t, ok)

			waits = append(waits, wait)
		}

		assert.Equal(t, []time.Duration{
			time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour,
		}, waits)
	})

	t.Run("treats a call before any failure as the first", func(t *testing.T) {
		t.Parallel()

		wait, ok := customdomain.DefaultSchedule().Next(0, 0)

		assert.True(t, ok)
		assert.Equal(t, time.Minute, wait)
	})

	t.Run("gives up once the next check would pass the deadline", func(t *testing.T) {
		t.Parallel()
		schedule := customdomain.DefaultSchedule()

		_, early := schedule.Next(40, 71*time.Hour)
		_, late := schedule.Next(40, 71*time.Hour+time.Minute)

		assert.True(t, early)
		assert.False(t, late)
	})
}
