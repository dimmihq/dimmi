package officehours_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/support/officehours"
)

func TestWithin(t *testing.T) {
	t.Parallel()

	week := json.RawMessage(`{"timezone":"Europe/Berlin","days":{"mon":{"open":"09:00","close":"17:00"},` +
		`"fri":{"open":"22:00","close":"06:00"}}}`)
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	at := func(day, hour, minute int) time.Time {
		return time.Date(2026, 11, day, hour, minute, 0, 0, berlin)
	}

	cases := []struct {
		name string
		t    time.Time
		want bool
	}{
		{"opens on a monday morning", at(2, 9, 0), true},
		{"closes at the close minute", at(2, 17, 0), false},
		{"is closed before opening", at(2, 8, 59), false},
		{"is closed on a day without a window", at(3, 12, 0), false},
		{"runs an overnight window into the evening", at(6, 23, 30), true},
		{"runs an overnight window past midnight", at(7, 5, 59), true},
		{"ends an overnight window at its close", at(7, 6, 0), false},
		{"reads the time in the configured zone", at(2, 9, 30).UTC(), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := officehours.Within(week, c.t)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}

	t.Run("fails closed without a setting", func(t *testing.T) {
		t.Parallel()
		got, err := officehours.Within(nil, at(2, 10, 0))
		assert.False(t, got)
		assert.ErrorIs(t, err, officehours.ErrNotConfigured)
	})

	invalid := map[string]string{
		"an unknown timezone": `{"timezone":"Mars/Base","days":{"mon":{"open":"09:00","close":"17:00"}}}`,
		"a missing timezone":  `{"days":{"mon":{"open":"09:00","close":"17:00"}}}`,
		"a bad time":          `{"timezone":"UTC","days":{"mon":{"open":"9am","close":"17:00"}}}`,
		"an unknown day":      `{"timezone":"UTC","days":{"someday":{"open":"09:00","close":"17:00"}}}`,
		"an empty window":     `{"timezone":"UTC","days":{"mon":{"open":"09:00","close":"09:00"}}}`,
		"broken json":         `{"timezone":`,
	}
	for name, raw := range invalid {
		t.Run("fails closed on "+name, func(t *testing.T) {
			t.Parallel()
			got, err := officehours.Within(json.RawMessage(raw), at(2, 10, 0))
			assert.False(t, got)
			assert.ErrorIs(t, err, officehours.ErrInvalid)
		})
	}
}
