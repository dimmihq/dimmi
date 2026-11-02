package foundation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/clock"
)

func TestClockOption(t *testing.T) {
	t.Parallel()

	t.Run("runs the whole application on the clock it is given", func(t *testing.T) {
		t.Parallel()
		frozen := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))
		opts := options(environ(t))
		opts.Clock = frozen

		app := newApp(t, opts)
		frozen.Travel(time.Hour)

		assert.Equal(t, time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC), app.Clock.Now())
	})
}
