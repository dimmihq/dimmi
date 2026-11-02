/*
|--------------------------------------------------------------------------
| Verification Backoff
|--------------------------------------------------------------------------
|
| DNS changes take minutes to hours to reach every resolver. A domain
| that does not verify yet is checked again soon, then less and less
| often, and given up on after a deadline so that a claim nobody follows
| through does not hold its hostname forever.
|
*/

package customdomain

import "time"

/*
Schedule says when to check a domain again after a failed check.

Steps are the waits after the first failures, in order. After the last
step the domain is checked every Every, until For has passed since the
first check; then the schedule gives up.
*/
type Schedule struct {
	Steps []time.Duration
	Every time.Duration
	For   time.Duration
}

/*
DefaultSchedule checks after 1, 2, 5, 10 and 30 minutes, then hourly,
and gives up 72 hours after the first check.
*/
func DefaultSchedule() Schedule {
	return Schedule{
		Steps: []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 30 * time.Minute},
		Every: time.Hour,
		For:   72 * time.Hour,
	}
}

/*
Next returns how long to wait before the next check, given how many
checks have failed so far and how long ago the first one ran. It
returns false when the schedule has given up.
*/
func (s Schedule) Next(failures int, elapsed time.Duration) (time.Duration, bool) {
	wait := s.Every
	if step := max(failures, 1) - 1; step < len(s.Steps) {
		wait = s.Steps[step]
	}

	if elapsed+wait > s.For {
		return 0, false
	}

	return wait, true
}
