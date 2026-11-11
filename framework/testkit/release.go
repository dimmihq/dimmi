/*
|--------------------------------------------------------------------------
| Releasing What A Test Held
|--------------------------------------------------------------------------
|
| The testing package keeps every cleanup a test registered after it ran,
| and keeps every finished parallel subtest until its parent test ends.
| A cleanup that holds an application, a server or a connection would keep
| it in memory for the rest of the run, so a package of parallel tests
| grows to many gigabytes. Cleanups registered here let go of what they
| hold once they ran, and memory stays at what the running tests need.
|
*/

package testkit

import "testing"

/*
Cleanup registers fn to run when the test ends, as tb.Cleanup does, and
drops fn once it ran, so nothing fn holds outlives the test.
*/
func Cleanup(tb testing.TB, fn func()) {
	tb.Helper()

	held := &fn
	tb.Cleanup(func() {
		run := *held
		*held = nil
		run()
	})
}
