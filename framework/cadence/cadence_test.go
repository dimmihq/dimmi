package cadence_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/cadence"
)

func parse(t *testing.T, kind, expr, zone string) *cadence.Schedule {
	t.Helper()
	s, err := cadence.Parse(cadence.Spec{Kind: kind, Expr: expr, Timezone: zone})
	require.NoError(t, err)
	return s
}

func at(t *testing.T, value string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return v
}

func TestPresets(t *testing.T) {
	t.Parallel()

	daily, err := cadence.Preset(cadence.Daily, cadence.At{Hour: 8, Minute: 30})
	require.NoError(t, err)
	assert.Equal(t, "30 8 * * *", daily)

	weekly, err := cadence.Preset(cadence.Weekly, cadence.At{Hour: 9, Weekday: 1})
	require.NoError(t, err)
	assert.Equal(t, "0 9 * * 1", weekly)

	last, err := cadence.Preset(cadence.Monthly, cadence.At{Hour: 9})
	require.NoError(t, err)
	assert.Equal(t, "0 9 L * *", last)

	_, err = cadence.Preset(cadence.Monthly, cadence.At{Hour: 9, MonthDay: 30})
	require.ErrorIs(t, err, cadence.ErrExpression, "days 29 to 31 are refused")
}

func TestNext(t *testing.T) {
	t.Parallel()

	t.Run("reads the wall clock of the routine's zone", func(t *testing.T) {
		t.Parallel()
		s := parse(t, cadence.Weekly, "0 9 * * 1", "America/New_York")
		assert.Equal(t, at(t, "2026-11-23T14:00:00Z"), s.Next(at(t, "2026-11-22T00:00:00Z")))
	})

	t.Run("runs daily", func(t *testing.T) {
		t.Parallel()
		s := parse(t, cadence.Daily, "30 8 * * *", "UTC")
		got := s.NextN(at(t, "2026-11-22T09:00:00Z"), 3)
		assert.Equal(t, []time.Time{
			at(t, "2026-11-23T08:30:00Z"), at(t, "2026-11-24T08:30:00Z"), at(t, "2026-11-25T08:30:00Z"),
		}, got)
	})

	t.Run("runs on the last day of each month", func(t *testing.T) {
		t.Parallel()
		s := parse(t, cadence.Monthly, "0 9 L * *", "Europe/Berlin")
		got := s.NextN(at(t, "2028-01-31T09:00:00Z"), 3)
		assert.Equal(t, []time.Time{
			at(t, "2028-02-29T08:00:00Z"), at(t, "2028-03-31T07:00:00Z"), at(t, "2028-04-30T07:00:00Z"),
		}, got, "a leap February, then summer time")
		assert.Equal(t, at(t, "2027-02-28T08:00:00Z"), s.Next(at(t, "2027-02-01T00:00:00Z")))
	})

	t.Run("runs a one-off routine once", func(t *testing.T) {
		t.Parallel()
		runAt := at(t, "2026-12-01T10:00:00Z")
		s, err := cadence.Parse(cadence.Spec{Kind: cadence.Once, RunAt: &runAt, Timezone: "UTC"})
		require.NoError(t, err)
		assert.Equal(t, runAt, s.Next(at(t, "2026-11-22T00:00:00Z")))
		assert.True(t, s.Next(runAt).IsZero())
	})

	t.Run("runs after a spring gap on the same day", func(t *testing.T) {
		t.Parallel()
		s := parse(t, cadence.Daily, "30 2 * * *", "America/New_York")
		next := s.Next(at(t, "2027-03-14T05:00:00Z"))
		local := next.In(s.Location())
		assert.Equal(t, 14, local.Day(), "the day of the gap is not skipped")
		assert.Equal(t, 3, local.Hour())
	})

	t.Run("runs once when autumn repeats the hour", func(t *testing.T) {
		t.Parallel()
		s := parse(t, cadence.Daily, "30 1 * * *", "America/New_York")
		first := s.Next(at(t, "2026-11-01T04:00:00Z"))
		assert.Equal(t, at(t, "2026-11-01T05:30:00Z"), first)
		second := s.Next(first)
		assert.Equal(t, 2, second.In(s.Location()).Day(), "the repeated 01:30 does not run again")
	})
}

func TestRefusals(t *testing.T) {
	t.Parallel()

	_, err := cadence.Parse(cadence.Spec{Kind: cadence.Cron, Expr: "0 9 * * *", Timezone: "Mars/Olympus"})
	require.ErrorIs(t, err, cadence.ErrTimezone)

	_, err = cadence.Parse(cadence.Spec{Kind: cadence.Cron, Expr: "every day", Timezone: "UTC"})
	require.ErrorIs(t, err, cadence.ErrExpression)

	_, err = cadence.Parse(cadence.Spec{Kind: cadence.Cron, Expr: "TZ=UTC 0 9 * * *", Timezone: "UTC"})
	require.ErrorIs(t, err, cadence.ErrExpression)

	_, err = cadence.Parse(cadence.Spec{Kind: cadence.Once, Timezone: "UTC"})
	require.ErrorIs(t, err, cadence.ErrRunAt)

	_, err = cadence.Parse(cadence.Spec{Kind: "hourly", Expr: "0 * * * *", Timezone: "UTC"})
	require.ErrorIs(t, err, cadence.ErrKind)
}

func TestCheckFrequency(t *testing.T) {
	t.Parallel()
	from := at(t, "2026-11-22T00:00:00Z")

	require.ErrorIs(t, parse(t, cadence.Cron, "*/15 * * * *", "UTC").CheckFrequency(from), cadence.ErrTooFrequent)
	require.ErrorIs(t, parse(t, cadence.Cron, "0,30 9 * * *", "UTC").CheckFrequency(from), cadence.ErrTooFrequent)
	require.NoError(t, parse(t, cadence.Cron, "0 * * * *", "UTC").CheckFrequency(from))
	require.NoError(t, parse(t, cadence.Weekly, "0 9 * * 1", "UTC").CheckFrequency(from))
}
