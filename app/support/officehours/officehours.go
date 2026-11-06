/*
|--------------------------------------------------------------------------
| Office Hours
|--------------------------------------------------------------------------
|
| The agent's office hours tell its schedule when "the team is around"
| is. The setting is a JSON week: per weekday one window "09:00" to
| "17:00" in a named timezone; a missing day is closed. A window whose
| close is not after its open runs past midnight into the next day, so
| a night shift is one window, not two.
|
| A setting that cannot be read never counts as open: every caller gets
| the same error and fails closed the same way.
|
*/

package officehours

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

/*
Hours is the stored office-hours shape. Days maps "mon" to "sun" to
that day's window.
*/
type Hours struct {
	Timezone string          `json:"timezone"`
	Days     map[string]Span `json:"days"`
}

/*
Span is one day's open window, "HH:MM" each side.
*/
type Span struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

/*
Errors a setting can fail with: ErrNotConfigured when there is none,
ErrInvalid when it cannot be read.
*/
var (
	ErrNotConfigured = errors.New("office hours not configured")
	ErrInvalid       = errors.New("office hours invalid")
)

/*
window is one parsed day: minutes past midnight each side.
*/
type window struct {
	open, close int
}

/*
Within reports whether t falls inside the hours. A missing or
unreadable setting answers false with an error, so a caller that only
looks at the boolean still fails closed.
*/
func Within(raw json.RawMessage, t time.Time) (bool, error) {
	loc, days, err := parse(raw)
	if err != nil {
		return false, err
	}

	local := t.In(loc)
	minutes := local.Hour()*60 + local.Minute()

	if w, ok := days[local.Weekday()]; ok {
		if w.close > w.open && minutes >= w.open && minutes < w.close {
			return true, nil
		}
		if w.close <= w.open && minutes >= w.open {
			return true, nil
		}
	}

	if w, ok := days[(local.Weekday()+6)%7]; ok && w.close <= w.open && minutes < w.close {
		return true, nil
	}

	return false, nil
}

/*
parse reads and validates the setting.
*/
func parse(raw json.RawMessage) (*time.Location, map[time.Weekday]window, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return nil, nil, ErrNotConfigured
	}

	var hours Hours
	if err := json.Unmarshal(raw, &hours); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	if hours.Timezone == "" {
		return nil, nil, fmt.Errorf("%w: no timezone", ErrInvalid)
	}
	loc, err := time.LoadLocation(hours.Timezone)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: timezone %q", ErrInvalid, hours.Timezone)
	}

	days := make(map[time.Weekday]window, len(hours.Days))
	for key, span := range hours.Days {
		day, ok := weekdays()[strings.ToLower(key)]
		if !ok {
			return nil, nil, fmt.Errorf("%w: day %q", ErrInvalid, key)
		}
		w, err := parseSpan(span)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %s: %w", ErrInvalid, key, err)
		}
		days[day] = w
	}

	return loc, days, nil
}

/*
parseSpan reads one window; open and close may not be equal.
*/
func parseSpan(s Span) (window, error) {
	open, err := minutesOf(s.Open)
	if err != nil {
		return window{}, err
	}
	closing, err := minutesOf(s.Close)
	if err != nil {
		return window{}, err
	}
	if open == closing {
		return window{}, errors.New("empty window")
	}
	return window{open: open, close: closing}, nil
}

/*
minutesOf parses "HH:MM" into minutes past midnight; "24:00" is the
end of the day.
*/
func minutesOf(s string) (int, error) {
	hh, mm, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return 0, fmt.Errorf("time %q", s)
	}
	h, herr := strconv.Atoi(hh)
	m, merr := strconv.Atoi(mm)
	if herr != nil || merr != nil || len(mm) != 2 || h < 0 || m < 0 || m > 59 || h > 24 || (h == 24 && m > 0) {
		return 0, fmt.Errorf("time %q", s)
	}
	return h*60 + m, nil
}

/*
weekdays maps the stored day keys.
*/
func weekdays() map[string]time.Weekday {
	return map[string]time.Weekday{
		"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
		"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
	}
}
