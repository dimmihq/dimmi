/*
|--------------------------------------------------------------------------
| Office Hours
|--------------------------------------------------------------------------
|
| The workspace's office hours tell the agent's schedule when "the
| team is around" is. The setting is a JSON week: per weekday either
| closed or one window "09:00-17:00" in a named timezone.
|
*/

package officehours

import (
	"encoding/json"
	"time"
)

/*
Hours is the stored office-hours shape.
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
Within reports whether t falls inside the hours; a missing or empty
setting answers true so the schedule is only restrictive when set.
*/
func Within(raw json.RawMessage, t time.Time) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return true
	}

	var hours Hours
	if err := json.Unmarshal(raw, &hours); err != nil {
		return true
	}

	loc, err := time.LoadLocation(hours.Timezone)
	if err != nil {
		loc = time.UTC
	}
	local := t.In(loc)

	day := local.Weekday().String()[:3]
	day = string(day[0]) + string(day[1]) + string(day[2])

	span, ok := hours.Days[weekday(local.Weekday())]
	if !ok {
		return false
	}

	minutes := local.Hour()*60 + local.Minute()
	return minutes >= hm(span.Open) && minutes < hm(span.Close)
}

/*
weekday names the day key.
*/
func weekday(d time.Weekday) string {
	switch d {
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	case time.Saturday:
		return "sat"
	}
	return "sun"
}

/*
hm parses "HH:MM" into minutes past midnight.
*/
func hm(s string) int {
	var h, m int
	_, _ = parseHM(s, &h, &m)
	return h*60 + m
}

/*
parseHM splits "HH:MM".
*/
func parseHM(s string, h, m *int) (int, error) {
	var hh, mm int
	for _, c := range s {
		if c == ':' {
			continue
		}
	}
	n, err := fmtSscanf(s, &hh, &mm)
	*h, *m = hh, mm
	return n, err
}

/*
fmtSscanf reads two ints from "HH:MM" without fmt.
*/
func fmtSscanf(s string, h, m *int) (int, error) {
	parts := splitOnce(s, ':')
	if len(parts) != 2 {
		return 0, errBadTime
	}
	*h = atoi(parts[0])
	*m = atoi(parts[1])
	return 2, nil
}

/*
splitOnce splits on the first rune.
*/
func splitOnce(s string, sep rune) []string {
	for i, c := range s {
		if c == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

/*
atoi parses a short decimal.
*/
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

/*
errBadTime marks a malformed span.
*/
var errBadTime = &badTime{"bad office-hours time"}

type badTime struct{ s string }

func (e *badTime) Error() string { return e.s }
