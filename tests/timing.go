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
| the race detector instead.
|
*/

package tests

import (
	"testing"
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
