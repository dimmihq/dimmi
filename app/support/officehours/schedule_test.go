package officehours_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
weekdays opens 09:00 to 17:30 Monday to Friday.
*/
func weekdays() officehours.Weekly {
	day := [][2]string{{"09:00", "17:30"}}

	return officehours.Weekly{"mon": day, "tue": day, "wed": day, "thu": day, "fri": day}
}

func mustSchedule(
	t *testing.T, tz string, w officehours.Weekly, holidays ...officehours.Holiday,
) *officehours.Schedule {
	t.Helper()
	s, err := officehours.ParseSchedule(tz, w, holidays)
	require.NoError(t, err)

	return s
}

func local(t *testing.T, tz, at string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation(tz)
	require.NoError(t, err)
	out, err := time.ParseInLocation("2006-01-02 15:04", at, loc)
	require.NoError(t, err)

	return out
}

func TestSchedule(t *testing.T) {
	t.Parallel()

	t.Run("is open inside a range and closed at its end", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, "Europe/Berlin", weekdays())

		assert.True(t, s.IsOpen(local(t, "Europe/Berlin", "2026-11-04 09:00")))
		assert.True(t, s.IsOpen(local(t, "Europe/Berlin", "2026-11-04 17:29")))
		assert.False(t, s.IsOpen(local(t, "Europe/Berlin", "2026-11-04 17:30")))
		assert.False(t, s.IsOpen(local(t, "Europe/Berlin", "2026-11-07 12:00")))
	})

	t.Run("finds the next opening past a weekend and a holiday", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, "Europe/Berlin", weekdays(), officehours.Holiday{Date: "2026-11-09", Name: "Closed"})

		next, err := s.NextOpen(local(t, "Europe/Berlin", "2026-11-07 10:00"))
		require.NoError(t, err)
		assert.Equal(t, local(t, "Europe/Berlin", "2026-11-10 09:00"), next)
	})

	t.Run("answers the moment itself when already open", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, "America/New_York", weekdays())
		now := local(t, "America/New_York", "2026-11-04 10:15")

		next, err := s.NextOpen(now)
		require.NoError(t, err)
		assert.Equal(t, now, next)
	})

	t.Run("keeps local opening times across daylight saving changes", func(t *testing.T) {
		t.Parallel()
		for _, c := range []struct{ tz, friday, monday string }{
			{"Europe/Berlin", "2026-03-27 18:00", "2026-03-30 09:00"},
			{"America/New_York", "2026-03-06 18:00", "2026-03-09 09:00"},
			{"Australia/Sydney", "2026-04-03 18:00", "2026-04-06 09:00"},
		} {
			s := mustSchedule(t, c.tz, weekdays())
			next, err := s.NextOpen(local(t, c.tz, c.friday))
			require.NoError(t, err)
			assert.Equal(t, local(t, c.tz, c.monday), next, c.tz)
		}
	})

	t.Run("counts only open time when adding business time", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, "Europe/Berlin", weekdays())

		due, err := s.AddBusiness(local(t, "Europe/Berlin", "2026-11-06 17:00"), time.Hour)
		require.NoError(t, err)
		assert.Equal(t, local(t, "Europe/Berlin", "2026-11-09 09:30"), due)

		due, err = s.AddBusiness(local(t, "Europe/Berlin", "2026-11-07 08:00"), 30*time.Minute)
		require.NoError(t, err)
		assert.Equal(t, local(t, "Europe/Berlin", "2026-11-09 09:30"), due)
	})

	t.Run("closes once across ranges that meet at midnight", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, "UTC", officehours.Weekly{
			"mon": {{"20:00", "24:00"}}, "tue": {{"00:00", "06:00"}, {"09:00", "10:00"}},
		})

		end, open := s.NextClose(local(t, "UTC", "2026-11-02 22:00"))
		require.True(t, open)
		assert.Equal(t, local(t, "UTC", "2026-11-03 06:00"), end)
	})

	t.Run("refuses ranges that end before they start, overlap or are too many", func(t *testing.T) {
		t.Parallel()
		for _, w := range []officehours.Weekly{
			{"mon": {{"17:00", "09:00"}}},
			{"mon": {{"09:00", "12:00"}, {"11:00", "13:00"}}},
			{"mon": {
				{"01:00", "02:00"}, {"03:00", "04:00"}, {"05:00", "06:00"}, {"07:00", "08:00"}, {"09:00", "10:00"},
			}},
			{"someday": {{"09:00", "10:00"}}},
		} {
			_, err := officehours.ParseSchedule("UTC", w, nil)
			assert.ErrorIs(t, err, officehours.ErrInvalid)
		}

		_, err := officehours.ParseSchedule("Mars/Olympus", weekdays(), nil)
		assert.ErrorIs(t, err, officehours.ErrInvalid)
	})

	t.Run("says when a schedule never opens", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, "UTC", officehours.Weekly{})

		_, err := s.NextOpen(local(t, "UTC", "2026-11-02 10:00"))
		assert.ErrorIs(t, err, officehours.ErrNeverOpen)
	})

	t.Run("reads a schedule as it is stored", func(t *testing.T) {
		t.Parallel()
		weekly, _ := json.Marshal(weekdays())
		s, err := officehours.ParseStored("Europe/Berlin", weekly, json.RawMessage(`[{"date":"2026-11-04"}]`))
		require.NoError(t, err)
		assert.False(t, s.IsOpen(local(t, "Europe/Berlin", "2026-11-04 10:00")))
		assert.Equal(t, "Europe/Berlin", s.Location().String())
	})
}
