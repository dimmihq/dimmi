package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/clock"
)

func TestFake(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	c := clock.NewFake(start)

	c.Travel(90 * time.Minute)
	assert.Equal(t, start.Add(90*time.Minute), c.Now())

	c.Freeze(start.Add(24 * time.Hour))
	assert.Equal(t, start.Add(24*time.Hour), c.Now())
}

func TestSystem(t *testing.T) {
	t.Parallel()

	assert.WithinDuration(t, clock.System().Now(), clock.System().Now(), time.Second)
}
