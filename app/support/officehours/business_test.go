package officehours_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/officehours"
)

func TestBusinessBetween(t *testing.T) {
	t.Parallel()

	const tz = "Europe/Berlin"

	cases := []struct {
		name     string
		from, to string
		holidays []officehours.Holiday
		want     time.Duration
	}{
		{"inside one range", "2026-11-04 10:00", "2026-11-04 11:15", nil, 75 * time.Minute},
		{"before opening counts from the open", "2026-11-04 07:00", "2026-11-04 09:30", nil, 30 * time.Minute},
		{"over a night", "2026-11-04 17:00", "2026-11-05 09:30", nil, time.Hour},
		{"over a weekend", "2026-11-06 17:00", "2026-11-09 10:00", nil, 90 * time.Minute},
		{"all weekend is nothing", "2026-11-07 08:00", "2026-11-08 20:00", nil, 0},
		{
			"over a holiday Monday", "2026-11-06 17:00", "2026-11-10 10:00",
			[]officehours.Holiday{{Date: "2026-11-09"}},
			90 * time.Minute,
		},
		{
			"a yearly holiday", "2026-12-24 17:00", "2026-12-28 09:15",
			[]officehours.Holiday{{Date: "2020-12-25", Yearly: true}},
			45 * time.Minute,
		},
		{"a whole week", "2026-11-02 00:00", "2026-11-09 00:00", nil, 5 * (8*time.Hour + 30*time.Minute)},
		{"an end before the start", "2026-11-04 11:00", "2026-11-04 10:00", nil, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := mustSchedule(t, tz, weekdays(), c.holidays...)

			assert.Equal(t, c.want, s.BusinessBetween(local(t, tz, c.from), local(t, tz, c.to)))
		})
	}

	t.Run("keeps wall times across a clock change", func(t *testing.T) {
		t.Parallel()
		day := [][2]string{{"00:00", "24:00"}}
		s := mustSchedule(t, tz, officehours.Weekly{"sun": day})

		got := s.BusinessBetween(local(t, tz, "2026-10-25 00:00"), local(t, tz, "2026-10-26 00:00"))

		assert.Equal(t, 25*time.Hour, got, "the day the clocks go back is 25 hours long")
	})
}
