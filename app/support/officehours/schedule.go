/*
|--------------------------------------------------------------------------
| Schedules
|--------------------------------------------------------------------------
|
| A schedule is a workspace's office hours as workflows use them: up to
| four open ranges per weekday in one timezone, and holidays closed the
| whole day. A range runs from its start to its end on the same day,
| "24:00" ending at midnight; a night shift is the end of one day and
| the start of the next.
|
| Every answer is worked out in the schedule's own timezone from local
| wall times, so a range keeps its local start and end across daylight
| saving changes. A moment a clock change skips is moved forward the
| way the timezone moves it.
|
*/

package officehours

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

/*
The limits of a schedule: ranges per day and how far ahead an answer
is looked for, past every holiday a year can hold, yearly ones
included, so a schedule closed every day of the year still ends the
search. Yearly holidays are matched by month and day alone.
*/
const (
	MaxRanges  = 4
	searchDays = 400
	monthDay   = "01-02"
)

/*
ErrNeverOpen is returned when a schedule has no open time to find.
*/
var ErrNeverOpen = errors.New("schedule is never open")

/*
Weekly maps "mon" to "sun" to the day's ranges, each a start and an
end as "HH:MM".
*/
type Weekly map[string][][2]string

/*
Holiday is a date, "2006-01-02", closed all day, with its name. A
yearly holiday closes the same month and day every year from any
year, so Christmas is entered once; a yearly February 29 closes only
in leap years. Holidays stored before the flag existed stay one-off.
*/
type Holiday struct {
	Date   string `json:"date"`
	Name   string `json:"name,omitempty"`
	Yearly bool   `json:"yearly,omitempty"`
}

/*
Schedule is a parsed schedule, ready to answer.
*/
type Schedule struct {
	loc      *time.Location
	days     map[time.Weekday][]window
	holidays map[string]bool
	yearly   map[string]bool
}

/*
ParseSchedule reads a schedule's timezone, week and holidays, refusing
a range that does not end after it starts, overlapping ranges, more
than four ranges a day and dates that are not dates.
*/
func ParseSchedule(timezone string, weekly Weekly, holidays []Holiday) (*Schedule, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil || timezone == "" {
		return nil, fmt.Errorf("%w: timezone %q", ErrInvalid, timezone)
	}

	s := &Schedule{
		loc:      loc,
		days:     make(map[time.Weekday][]window, len(weekly)),
		holidays: map[string]bool{},
		yearly:   map[string]bool{},
	}
	for key, ranges := range weekly {
		day, ok := weekdays()[key]
		if !ok {
			return nil, fmt.Errorf("%w: day %q", ErrInvalid, key)
		}

		windows, err := parseRanges(ranges)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrInvalid, key, err)
		}

		s.days[day] = windows
	}

	for _, h := range holidays {
		date, err := time.Parse(time.DateOnly, h.Date)
		if err != nil {
			return nil, fmt.Errorf("%w: holiday %q", ErrInvalid, h.Date)
		}

		if h.Yearly {
			s.yearly[date.Format(monthDay)] = true

			continue
		}

		s.holidays[h.Date] = true
	}

	return s, nil
}

/*
ParseStored reads a schedule as it is stored: the week and holidays as
JSON.
*/
func ParseStored(timezone string, weekly, holidays json.RawMessage) (*Schedule, error) {
	var w Weekly
	if err := json.Unmarshal(weekly, &w); err != nil {
		return nil, fmt.Errorf("%w: week: %w", ErrInvalid, err)
	}

	var h []Holiday
	if len(holidays) > 0 {
		if err := json.Unmarshal(holidays, &h); err != nil {
			return nil, fmt.Errorf("%w: holidays: %w", ErrInvalid, err)
		}
	}

	return ParseSchedule(timezone, w, h)
}

/*
parseRanges reads one day's ranges, sorted by start.
*/
func parseRanges(ranges [][2]string) ([]window, error) {
	if len(ranges) > MaxRanges {
		return nil, errors.New("more than four ranges")
	}

	out := make([]window, 0, len(ranges))
	for _, r := range ranges {
		start, err := minutesOf(r[0])
		if err != nil {
			return nil, err
		}

		end, err := endMinutes(r[1])
		if err != nil {
			return nil, err
		}

		if end <= start {
			return nil, fmt.Errorf("range %s to %s ends before it starts", r[0], r[1])
		}

		out = append(out, window{open: start, close: end})
	}

	slices.SortFunc(out, func(a, b window) int { return a.open - b.open })
	for i := 1; i < len(out); i++ {
		if out[i].open < out[i-1].close {
			return nil, errors.New("ranges overlap")
		}
	}

	return out, nil
}

/*
endMinutes reads a range's end, where "24:00" is midnight at the end of
the day.
*/
func endMinutes(s string) (int, error) {
	if s == "24:00" {
		return 24 * 60, nil
	}

	return minutesOf(s)
}

/*
Location is the schedule's timezone.
*/
func (s *Schedule) Location() *time.Location {
	return s.loc
}

/*
IsOpen reports whether the schedule is open at the moment.
*/
func (s *Schedule) IsOpen(t time.Time) bool {
	_, ok := s.rangeAt(t)

	return ok
}

/*
NextOpen returns the moment the schedule next opens: the moment itself
when it is open.
*/
func (s *Schedule) NextOpen(t time.Time) (time.Time, error) {
	if s.IsOpen(t) {
		return t, nil
	}

	local := t.In(s.loc)
	for offset := range searchDays {
		day := time.Date(local.Year(), local.Month(), local.Day()+offset, 0, 0, 0, 0, s.loc)
		for _, w := range s.windowsOn(day) {
			if start := at(day, w.open); start.After(t) {
				return start, nil
			}
		}
	}

	return time.Time{}, ErrNeverOpen
}

/*
NextClose returns when the open range the moment falls in ends, and
false when the schedule is closed then. Ranges that meet, such as a
day ending at midnight and the next starting at midnight, close once.
*/
func (s *Schedule) NextClose(t time.Time) (time.Time, bool) {
	end, ok := s.rangeAt(t)
	if !ok {
		return time.Time{}, false
	}

	for range searchDays {
		next, open := s.rangeAt(end)
		if !open || !next.After(end) {
			return end, true
		}

		end = next
	}

	return end, true
}

/*
AddBusiness returns the moment the given open time has passed after
the start, counting only the time the schedule is open.
*/
func (s *Schedule) AddBusiness(start time.Time, d time.Duration) (time.Time, error) {
	cur := start
	for range searchDays * MaxRanges {
		open, err := s.NextOpen(cur)
		if err != nil {
			return time.Time{}, err
		}

		end, _ := s.NextClose(open)
		if d <= end.Sub(open) {
			return open.Add(d), nil
		}

		d -= end.Sub(open)
		cur = end
	}

	return time.Time{}, ErrNeverOpen
}

/*
rangeAt returns the end of the open range the moment falls in.
*/
func (s *Schedule) rangeAt(t time.Time) (time.Time, bool) {
	local := t.In(s.loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
	for _, w := range s.windowsOn(day) {
		if !t.Before(at(day, w.open)) && t.Before(at(day, w.close)) {
			return at(day, w.close), true
		}
	}

	return time.Time{}, false
}

/*
windowsOn returns the ranges of a local day, none on a holiday, dated
or yearly.
*/
func (s *Schedule) windowsOn(day time.Time) []window {
	if s.holidays[day.Format(time.DateOnly)] || s.yearly[day.Format(monthDay)] {
		return nil
	}

	return s.days[day.Weekday()]
}

/*
at returns the local wall time minutes past the day's midnight.
*/
func at(day time.Time, minutes int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), minutes/60, minutes%60, 0, 0, day.Location())
}

/*
BusinessBetween returns how long the schedule was open between two
moments, nothing when the end does not come after the start. Each local
day the span touches adds the part of its ranges inside the span, so
weekends, holidays and closed nights count for nothing, and a range
keeps its wall times across a daylight saving change.
*/
func (s *Schedule) BusinessBetween(from, to time.Time) time.Duration {
	if !to.After(from) {
		return 0
	}

	var total time.Duration

	local := from.In(s.loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
	for !day.After(to) {
		for _, w := range s.windowsOn(day) {
			open, end := at(day, w.open), at(day, w.close)
			if open.Before(from) {
				open = from
			}

			if end.After(to) {
				end = to
			}

			if end.After(open) {
				total += end.Sub(open)
			}
		}

		day = time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, s.loc)
	}

	return total
}
