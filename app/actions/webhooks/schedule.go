/*
|--------------------------------------------------------------------------
| Retry Schedule
|--------------------------------------------------------------------------
|
| A delivery is tried twelve times over a day, each time counted from
| the first attempt and spread by a fifth either way, so endpoints that
| failed together do not all come back at the same second. The spread
| is drawn from the delivery's id and the attempt's number, so it is the
| same every time it is computed. A receiver that asks to be left alone
| for a while with Retry-After is, unless that runs past the day.
|
*/

package webhooks

import (
	"encoding/binary"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Attempt triggers: the schedule, a redelivery someone asked for, and a
test send.
*/
const (
	triggerAuto   = "auto"
	triggerManual = "manual"
	triggerTest   = "test"
)

/*
Bounds of the schedule: the spread either way, the shortest gap between
two attempts, and the end of the window.
*/
const (
	jitter     = 0.2
	minimumGap = 5 * time.Second
	window     = 24 * time.Hour
)

/*
retryOffsets is when each attempt is due, counted from the first.
*/
func retryOffsets() []time.Duration {
	return []time.Duration{
		0, 30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute, time.Hour, 2 * time.Hour,
		4 * time.Hour, 8 * time.Hour, 14 * time.Hour, 20 * time.Hour, window,
	}
}

/*
MaxAttempts is how many times the schedule tries a delivery.
*/
func MaxAttempts() int {
	return len(retryOffsets())
}

/*
nextAttempt returns when the attempt after the done-th is due, or false
when the schedule has none left. The last answer r sets the earliest
time: never sooner than the minimum gap after it, nor before the wait
the receiver asked for.
*/
func nextAttempt(id ids.UUID, first time.Time, done int, r sent) (time.Time, bool) {
	offsets := retryOffsets()
	if done >= len(offsets) {
		return time.Time{}, false
	}

	offset := offsets[done]
	if offset < window {
		offset = time.Duration(float64(offset) * (1 + spread(id, done)))
	}

	due := first.Add(offset)
	if earliest := r.at.Add(minimumGap); due.Before(earliest) {
		due = earliest
	}

	if asked := r.at.Add(r.retryAfter); asked.After(due) {
		due = asked
	}

	if end := first.Add(window); due.After(end) {
		due = end
	}

	return due, true
}

/*
spread returns a number in [-jitter, jitter) drawn from the delivery and
the attempt.
*/
func spread(id ids.UUID, n int) float64 {
	h := fnv.New64a()
	_, _ = h.Write(id[:])
	_ = binary.Write(h, binary.BigEndian, int64(n))

	unit := float64(h.Sum64()>>11) / float64(1<<53)

	return (unit*2 - 1) * jitter
}

/*
parseRetryAfter reads a Retry-After value in seconds or as an HTTP date,
zero for anything else or a time already passed.
*/
func parseRetryAfter(value string, now time.Time) time.Duration {
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		return max(time.Duration(seconds)*time.Second, 0)
	}

	at, err := time.Parse(time.RFC1123, value)
	if err != nil || !at.After(now) {
		return 0
	}

	return at.Sub(now)
}
