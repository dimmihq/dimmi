package eventroll_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/eventroll"
)

func TestSummaries(t *testing.T) {
	t.Parallel()

	monday := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	days := func(n int) time.Time { return monday.AddDate(0, 0, n) }

	t.Run("counts events in the days they happened, as days pass", func(t *testing.T) {
		t.Parallel()
		s := eventroll.Add(eventroll.Summary{}, monday, 1, monday)
		s = eventroll.Add(s, days(2), 2, days(2))
		s = eventroll.Add(s, days(9), 1, days(9))

		assert.Equal(t, int64(4), s.Count)
		assert.Equal(t, int64(1), eventroll.Within(s, 7, days(9)))
		assert.Equal(t, int64(3), eventroll.Within(s, 8, days(9)))
		assert.Equal(t, int64(4), eventroll.Within(s, 10, days(9)))
		assert.Equal(t, int64(0), eventroll.Within(s, 7, days(20)))
	})

	t.Run("keeps first and last whatever order events arrive in", func(t *testing.T) {
		t.Parallel()
		s := eventroll.Add(eventroll.Summary{}, days(3), 1, days(3))
		s = eventroll.Add(s, days(1), 1, days(3))

		assert.Equal(t, days(1), s.FirstAt)
		assert.Equal(t, days(3), s.LastAt)
		assert.Equal(t, int64(2), eventroll.Within(s, 3, days(3)))
	})

	t.Run("counts old events in the total only and caps a day", func(t *testing.T) {
		t.Parallel()
		s := eventroll.Add(eventroll.Summary{}, days(-120), 1, monday)
		s = eventroll.Add(s, monday, 40000, monday)

		assert.Equal(t, int64(40001), s.Count)
		assert.Equal(t, int64(32767), eventroll.Within(s, 90, monday))
	})

	t.Run("combines two people's summaries day by day", func(t *testing.T) {
		t.Parallel()
		a := eventroll.Add(eventroll.Summary{}, monday, 2, monday)
		b := eventroll.Add(eventroll.Summary{}, days(3), 1, days(3))
		b = eventroll.Add(b, days(-5), 1, days(3))

		c := eventroll.Combine(a, b, days(4))
		assert.Equal(t, int64(4), c.Count)
		assert.Equal(t, days(-5), c.FirstAt)
		assert.Equal(t, days(3), c.LastAt)
		assert.Equal(t, int64(3), eventroll.Within(c, 5, days(4)))
		assert.Equal(t, int64(4), eventroll.Within(c, 10, days(4)))
		assert.Equal(t, int64(2), eventroll.Combine(eventroll.Summary{}, a, monday).Count)
	})
}
