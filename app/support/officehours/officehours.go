/*
|--------------------------------------------------------------------------
| Office Hours
|--------------------------------------------------------------------------
|
| The pieces every schedule is read with: wall times as "HH:MM", where
| "24:00" is the end of the day, the weekday keys "mon" to "sun", and
| the one error a schedule that cannot be read fails with, so every
| caller fails closed the same way.
|
*/

package officehours

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

/*
ErrInvalid is returned for a schedule that cannot be read.
*/
var ErrInvalid = errors.New("office hours invalid")

/*
window is one open range of a day: minutes past midnight each side.
*/
type window struct {
	open, close int
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
