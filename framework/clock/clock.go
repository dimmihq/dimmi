/*
|--------------------------------------------------------------------------
| Clock
|--------------------------------------------------------------------------
|
| Business rules depend on time: reminders fire, SLAs breach, tokens expire.
| Code asks a clock for the time instead of the operating system, so tests
| can freeze it at a known moment and move it forward without sleeping.
|
*/

package clock

import (
	"time"

	"github.com/jonboulle/clockwork"
)

/*
Clock tells the time and waits for it to pass.
*/
type Clock = clockwork.Clock

/*
Fake is a clock that moves only when a test moves it.
*/
type Fake struct {
	*clockwork.FakeClock
}

/*
System returns the real wall clock.
*/
func System() Clock {
	return clockwork.NewRealClock()
}

/*
NewFake returns a fake clock frozen at the given time.
*/
func NewFake(now time.Time) *Fake {
	return &Fake{FakeClock: clockwork.NewFakeClockAt(now)}
}

/*
Freeze sets the time. Timers due by then fire.
*/
func (f *Fake) Freeze(t time.Time) {
	f.Advance(t.Sub(f.Now()))
}

/*
Travel moves the time forward. Timers that come due fire.
*/
func (f *Fake) Travel(d time.Duration) {
	f.Advance(d)
}
