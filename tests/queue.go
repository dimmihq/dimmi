/*
|--------------------------------------------------------------------------
| Waiting Jobs
|--------------------------------------------------------------------------
|
| Some jobs are queued to run a little later, such as an email that waits
| to see whether the teammate reads the conversation first. Tests do not
| sleep through the wait: they let every waiting job run now.
|
*/

package tests

import "github.com/stretchr/testify/require"

/*
DrainLater makes every waiting job due now and drains the queue. A
waiting job is a scheduled one, or one River's scheduler already made
available because it comes due within its next pass, but whose time is
still ahead.
*/
func (tc *TestCase) DrainLater() {
	tc.T.Helper()
	tc.Drain()

	_, err := tc.Operator().Exec(tc.T.Context(),
		`UPDATE river_job SET state = 'available', scheduled_at = now()
		WHERE state = 'scheduled' OR state = 'available' AND scheduled_at > now()`)
	require.NoError(tc.T, err)
	tc.Drain()
}
