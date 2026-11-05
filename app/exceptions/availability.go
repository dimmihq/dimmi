/*
|--------------------------------------------------------------------------
| Availability Failures
|--------------------------------------------------------------------------
|
| Refusals of a change to how available a member is. Handing customers'
| answers back to the pool only means something while away, so asking for
| it while online is refused rather than stored and forgotten.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Availability failures.

ErrAwayReassignOnline refuses reassigning while away for a member who
stays online.
*/
var (
	ErrAwayReassignOnline = fault.New(fault.Invalid, "away_reassign_requires_away",
		"Conversations can only go back to the team while you are away.").OnField("away_reassign")
)
