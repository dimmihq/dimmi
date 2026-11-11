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

/*
everyDay opens 09:00 to 17:00 on all seven days, so only holidays
close a day.
*/
func everyDay() officehours.Weekly {
	day := [][2]string{{"09:00", "17:00"}}

	return officehours.Weekly{"mon": day, "tue": day, "wed": day, "thu": day, "fri": day, "sat": day, "sun": day}
}

func TestYearlyHolidays(t *testing.T) {
	t.Parallel()

	const tz = "Europe/Berlin"

	t.Run("closes the same month and day every year", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, tz, everyDay(), officehours.Holiday{Date: "2026-12-25", Name: "Christmas", Yearly: true})

		assert.False(t, s.IsOpen(local(t, tz, "2026-12-25 12:00")))
		assert.False(t, s.IsOpen(local(t, tz, "2031-12-25 12:00")))
		assert.False(t, s.IsOpen(local(t, tz, "2020-12-25 12:00")))
		assert.True(t, s.IsOpen(local(t, tz, "2027-12-24 12:00")))
		assert.True(t, s.IsOpen(local(t, tz, "2027-12-26 12:00")))
	})

	t.Run("keeps a holiday that is not yearly to its one date", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, tz, everyDay(), officehours.Holiday{Date: "2026-12-25"})

		assert.False(t, s.IsOpen(local(t, tz, "2026-12-25 12:00")))
		assert.True(t, s.IsOpen(local(t, tz, "2027-12-25 12:00")))
	})

	t.Run("closes a yearly february 29 only in leap years", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, tz, everyDay(), officehours.Holiday{Date: "2028-02-29", Yearly: true})

		assert.False(t, s.IsOpen(local(t, tz, "2032-02-29 12:00")))
		assert.True(t, s.IsOpen(local(t, tz, "2027-02-28 12:00")))
		assert.True(t, s.IsOpen(local(t, tz, "2027-03-01 12:00")))

		next, err := s.NextOpen(local(t, tz, "2032-02-28 18:00"))
		require.NoError(t, err)
		assert.Equal(t, local(t, tz, "2032-03-01 09:00"), next)
	})

	t.Run("finds the next opening past holidays that span the year end", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, tz, everyDay(),
			officehours.Holiday{Date: "2026-12-31", Name: "New Year's Eve", Yearly: true},
			officehours.Holiday{Date: "2027-01-01", Name: "New Year", Yearly: true},
		)

		next, err := s.NextOpen(local(t, tz, "2029-12-30 18:00"))
		require.NoError(t, err)
		assert.Equal(t, local(t, tz, "2030-01-02 09:00"), next)

		end, err := s.AddBusiness(local(t, tz, "2029-12-30 16:00"), 2*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, local(t, tz, "2030-01-02 10:00"), end)
	})

	t.Run("accepts a yearly holiday and a dated one on the same day", func(t *testing.T) {
		t.Parallel()
		s := mustSchedule(t, tz, everyDay(),
			officehours.Holiday{Date: "2026-12-25", Yearly: true},
			officehours.Holiday{Date: "2026-12-25"},
		)

		assert.False(t, s.IsOpen(local(t, tz, "2026-12-25 12:00")))
		assert.False(t, s.IsOpen(local(t, tz, "2027-12-25 12:00")))
	})

	t.Run("stops looking when yearly holidays close every day", func(t *testing.T) {
		t.Parallel()
		var all []officehours.Holiday
		for day := time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC); day.Year() == 2028; day = day.AddDate(0, 0, 1) {
			all = append(all, officehours.Holiday{Date: day.Format(time.DateOnly), Yearly: true})
		}

		s := mustSchedule(t, tz, everyDay(), all...)

		_, err := s.NextOpen(local(t, tz, "2026-11-11 12:00"))
		assert.ErrorIs(t, err, officehours.ErrNeverOpen)
	})

	t.Run("reads holidays stored before the flag as one-off", func(t *testing.T) {
		t.Parallel()
		weekly, _ := json.Marshal(everyDay())
		s, err := officehours.ParseStored(tz, weekly, json.RawMessage(`[{"date":"2026-12-25","name":"Christmas"}]`))
		require.NoError(t, err)

		assert.False(t, s.IsOpen(local(t, tz, "2026-12-25 12:00")))
		assert.True(t, s.IsOpen(local(t, tz, "2027-12-25 12:00")))
	})

	t.Run("refuses a yearly holiday that is not a date", func(t *testing.T) {
		t.Parallel()
		_, err := officehours.ParseSchedule(tz, everyDay(), []officehours.Holiday{{Date: "2027-02-29", Yearly: true}})
		assert.ErrorIs(t, err, officehours.ErrInvalid)
	})
}
