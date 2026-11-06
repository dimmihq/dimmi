/*
|--------------------------------------------------------------------------
| Timing Checks
|--------------------------------------------------------------------------
|
| A few tests prove that two answers take the same time, so nobody can
| tell an unknown account from a wrong password by the clock. The race
| detector slows memory access many times over, so work that costs a
| millisecond in production costs tens of them under it, and those
| comparisons stop meaning anything. They run in their own pass without
| the race detector instead. A shared test machine is noisier than a
| server, so two medians may differ by ten milliseconds or a fifth of the
| slower one, whichever is more: noise stays inside that, while skipping a
| password hash or a database read for one kind of answer does not.
|
*/

package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

/*
RequireTimingAccuracy skips the test when the race detector is on, since
it distorts how long each request takes too much to compare them.
*/
func RequireTimingAccuracy(t *testing.T) {
	t.Helper()

	if raceDetector {
		t.Skip("timings are only compared without the race detector")
	}
}

/*
AssertSameTiming fails the test when the two medians differ by more than
ten milliseconds or a fifth of the slower one, whichever is more.
*/
func AssertSameTiming(t *testing.T, first, second time.Duration, names ...string) {
	t.Helper()

	allowed := max(10*time.Millisecond, max(first, second)/5)
	assert.Less(t, (first - second).Abs(), allowed, "%v: %s and %s", names, first, second)
}
