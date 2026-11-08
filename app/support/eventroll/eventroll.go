/*
|--------------------------------------------------------------------------
| Event Summaries
|--------------------------------------------------------------------------
|
| No raw event is stored. A person's summary of one event holds the total,
| the first and last time, and one count for each of the last 90 UTC
| days, the newest first. That answers "did it ever", "how long ago" and
| "how often in the last N days" for any N up to 90. An event older than
| 90 days still counts in the total and the dates, but in no day. Each
| day holds 32,767 at most.
|
*/

package eventroll

import (
	"math"
	"time"
)

/*
Days is how many days of counts a summary keeps.
*/
const Days = 90

/*
Summary is a person's running summary of one event. Day is the UTC
midnight that Daily[0] counts; Daily[i] counts i days before it.
*/
type Summary struct {
	Count   int64
	FirstAt time.Time
	LastAt  time.Time
	Day     time.Time
	Daily   []int16
}

/*
Add counts n events that happened at the time, the summary's days moved
on to now's day.
*/
func Add(s Summary, at time.Time, n int, now time.Time) Summary {
	today := day(now)
	if s.Count == 0 || s.Day.IsZero() {
		s = Summary{Day: today, Daily: make([]int16, Days), FirstAt: at, LastAt: at}
	}

	s = moveTo(s, today)

	if i := daysBetween(day(at), today); i >= 0 && i < Days {
		s.Daily[i] = clamp(int64(s.Daily[i]) + int64(n))
	}

	s.Count += int64(n)
	if at.Before(s.FirstAt) {
		s.FirstAt = at
	}

	if at.After(s.LastAt) {
		s.LastAt = at
	}

	return s
}

/*
Within counts the events of the last days, today included, as of now.
*/
func Within(s Summary, days int, now time.Time) int64 {
	shift := daysBetween(s.Day, day(now))
	var total int64

	for i, n := range s.Daily {
		if age := i + shift; age >= 0 && age < days {
			total += int64(n)
		}
	}

	return total
}

/*
moveTo shifts the daily counts so Daily[0] is the day given.
*/
func moveTo(s Summary, today time.Time) Summary {
	shift := daysBetween(s.Day, today)
	if shift <= 0 {
		return s
	}

	moved := make([]int16, Days)
	for i, n := range s.Daily {
		if j := i + shift; j < Days {
			moved[j] = n
		}
	}

	s.Daily, s.Day = moved, today

	return s
}

func day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func daysBetween(from, to time.Time) int {
	return int(math.Round(to.Sub(from).Hours() / 24))
}

func clamp(n int64) int16 {
	if n >= math.MaxInt16 {
		return math.MaxInt16
	}

	if n <= 0 {
		return 0
	}

	return int16(n)
}
