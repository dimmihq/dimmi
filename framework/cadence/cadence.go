/*
|--------------------------------------------------------------------------
| Cadence
|--------------------------------------------------------------------------
|
| When something that repeats runs next, in its own time zone. Daily,
| weekly and monthly cadences are cron expressions; a custom cron runs
| at most once an hour. A monthly cadence can run on the last day of the
| month, written L in the day field, since days 29 to 31 do not exist
| every month. A one-off cadence runs once.
|
| Times are read on the wall clock of the zone. When a spring change
| skips the hour it runs at, it runs just after the gap that day; when
| an autumn change repeats that hour, it runs once.
|
*/

package cadence

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

/*
The kinds of cadence.
*/
const (
	Daily   = "daily"
	Weekly  = "weekly"
	Monthly = "monthly"
	Cron    = "cron"
	Once    = "once"
)

/*
Kinds lists every kind.
*/
func Kinds() []string { return []string{Daily, Weekly, Monthly, Cron, Once} }

/*
MinInterval is the shortest gap a routine allows between two runs.
*/
const MinInterval = time.Hour

/*
The reasons a cadence is refused.
*/
var (
	ErrKind        = errors.New("cadence: unknown kind")
	ErrExpression  = errors.New("cadence: invalid cron expression")
	ErrTimezone    = errors.New("cadence: unknown time zone")
	ErrRunAt       = errors.New("cadence: a one-off routine needs a time to run at")
	ErrTooFrequent = errors.New("cadence: runs more often than once an hour")
)

/*
Spec is a routine's cadence as stored: its kind, the cron expression of
a repeating one, the moment of a one-off one, and the zone it reads its
wall clock in.
*/
type Spec struct {
	Kind     string
	Expr     string
	RunAt    *time.Time
	Timezone string
}

/*
Schedule finds a routine's runs.
*/
type Schedule struct {
	kind  string
	runAt time.Time
	loc   *time.Location
	cron  cron.Schedule
	last  *lastDay
}

/*
lastDay is a monthly run on the month's last day at one wall time.
*/
type lastDay struct {
	hour, minute int
	months       map[time.Month]bool
}

/*
parser reads five-field cron expressions.
*/
func parser() cron.Parser {
	return cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
}

/*
Parse checks a cadence and returns its schedule.
*/
func Parse(s Spec) (*Schedule, error) {
	loc, err := time.LoadLocation(strings.TrimSpace(s.Timezone))
	if err != nil || s.Timezone == "" {
		return nil, ErrTimezone
	}
	out := &Schedule{kind: s.Kind, loc: loc}
	switch s.Kind {
	case Once:
		if s.RunAt == nil || s.RunAt.IsZero() {
			return nil, ErrRunAt
		}
		out.runAt = s.RunAt.UTC()
		return out, nil
	case Daily, Weekly, Monthly, Cron:
	default:
		return nil, ErrKind
	}
	expr := strings.Join(strings.Fields(s.Expr), " ")
	if strings.Contains(expr, "TZ=") || strings.HasPrefix(expr, "@") {
		return nil, ErrExpression
	}
	if fields := strings.Fields(expr); len(fields) == 5 && fields[2] == "L" {
		out.last, err = parseLastDay(fields)
		return out, err
	}
	if out.cron, err = parser().Parse(expr); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrExpression, err.Error())
	}
	return out, nil
}

/*
parseLastDay reads "M H L months *": a fixed minute and hour on the
last day of the months named.
*/
func parseLastDay(fields []string) (*lastDay, error) {
	minute, err1 := strconv.Atoi(fields[0])
	hour, err2 := strconv.Atoi(fields[1])
	if err1 != nil || err2 != nil || minute < 0 || minute > 59 || hour < 0 || hour > 23 || fields[4] != "*" {
		return nil, fmt.Errorf("%w: the last day takes a fixed minute and hour", ErrExpression)
	}
	months := make(map[time.Month]bool, 12)
	if fields[3] == "*" {
		for m := time.January; m <= time.December; m++ {
			months[m] = true
		}
		return &lastDay{hour: hour, minute: minute, months: months}, nil
	}
	for part := range strings.SplitSeq(fields[3], ",") {
		m, err := strconv.Atoi(part)
		if err != nil || m < 1 || m > 12 {
			return nil, fmt.Errorf("%w: months are 1 to 12", ErrExpression)
		}
		months[time.Month(m)] = true
	}
	return &lastDay{hour: hour, minute: minute, months: months}, nil
}

/*
Location is the zone the schedule reads its wall clock in.
*/
func (s *Schedule) Location() *time.Location { return s.loc }

/*
Next returns the first run after the moment, in UTC, or the zero time
when there is none.
*/
func (s *Schedule) Next(after time.Time) time.Time {
	switch {
	case s.kind == Once:
		if s.runAt.After(after) {
			return s.runAt
		}
		return time.Time{}
	case s.last != nil:
		return s.last.next(after.In(s.loc)).UTC()
	}
	naive := s.cron.Next(onWall(after.In(s.loc), time.UTC))
	for range 4 {
		if naive.IsZero() {
			return naive
		}
		if at := onWall(naive, s.loc); at.After(after) {
			return at.UTC()
		}
		naive = s.cron.Next(naive)
	}
	return time.Time{}
}

/*
onWall is the moment showing the same wall clock in another zone. Cron
runs on a zone without changes, so a time a spring change skips moves
past the gap and a time an autumn change repeats comes once.
*/
func onWall(t time.Time, loc *time.Location) time.Time {
	out := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), loc)
	if out.Hour() == t.Hour() && out.Minute() == t.Minute() {
		return out
	}
	_, before := time.Date(t.Year(), t.Month(), t.Day(), t.Hour()-3, t.Minute(), 0, 0, loc).Zone()
	naive := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	return naive.Add(-time.Duration(before) * time.Second).In(loc)
}

/*
NextN returns the next n runs after the moment.
*/
func (s *Schedule) NextN(after time.Time, n int) []time.Time {
	var out []time.Time
	for range n {
		next := s.Next(after)
		if next.IsZero() {
			break
		}
		out = append(out, next)
		after = next
	}
	return out
}

/*
CheckFrequency refuses a schedule whose runs come closer than the
minimum interval anywhere in the week after the moment.
*/
func (s *Schedule) CheckFrequency(from time.Time) error {
	if s.kind == Once || s.last != nil {
		return nil
	}
	end := from.Add(8 * 24 * time.Hour)
	prev := s.Next(from)
	for i := 0; i < 24*8 && !prev.IsZero() && prev.Before(end); i++ {
		next := s.Next(prev)
		if next.IsZero() {
			return nil
		}
		if next.Sub(prev) < MinInterval {
			return ErrTooFrequent
		}
		prev = next
	}
	return nil
}

/*
next is the first last-day run after the local moment.
*/
func (l *lastDay) next(after time.Time) time.Time {
	loc := after.Location()
	year, month := after.Year(), after.Month()
	for range 25 {
		if l.months[month] {
			day := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
			at := time.Date(year, month, day, l.hour, l.minute, 0, 0, loc)
			if at.After(after) {
				return at
			}
		}
		month++
		if month > time.December {
			month, year = time.January, year+1
		}
	}
	return time.Time{}
}

/*
At is the wall time of a daily, weekly or monthly cadence: Weekday 0
to 6 from Sunday for weekly, MonthDay 1 to 28 for monthly, or 0 for the
month's last day.
*/
type At struct {
	Hour, Minute, Weekday, MonthDay int
}

/*
Preset is the cron expression of a daily, weekly or monthly cadence.
*/
func Preset(kind string, at At) (string, error) {
	if at.Hour < 0 || at.Hour > 23 || at.Minute < 0 || at.Minute > 59 {
		return "", fmt.Errorf("%w: hour 0 to 23 and minute 0 to 59", ErrExpression)
	}
	switch kind {
	case Daily:
		return fmt.Sprintf("%d %d * * *", at.Minute, at.Hour), nil
	case Weekly:
		if at.Weekday < 0 || at.Weekday > 6 {
			return "", fmt.Errorf("%w: weekday 0 to 6", ErrExpression)
		}
		return fmt.Sprintf("%d %d * * %d", at.Minute, at.Hour, at.Weekday), nil
	case Monthly:
		if at.MonthDay == 0 {
			return fmt.Sprintf("%d %d L * *", at.Minute, at.Hour), nil
		}
		if at.MonthDay < 1 || at.MonthDay > 28 {
			return "", fmt.Errorf("%w: day of month 1 to 28, or the last day", ErrExpression)
		}
		return fmt.Sprintf("%d %d %d * *", at.Minute, at.Hour, at.MonthDay), nil
	}
	return "", ErrKind
}
