/*
|--------------------------------------------------------------------------
| History Import Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a developer brings conversations in from
| another tool with the times they really happened. A time is refused
| when it lies ahead, when it is older than any help desk worth
| importing from, or when it would land before what the conversation
| already holds, since history is written oldest first.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
ErrImportTimeInFuture is a time that has not come yet.
ErrImportTimeTooEarly is a time before the year 2000.
ErrImportOutOfOrder is a time before the latest entry of the
conversation. ErrImportClosedBeforeStart is a close before the
conversation's first message.
*/
var (
	ErrImportTimeInFuture = fault.New(fault.Invalid, "import_time_in_future",
		"The time cannot be in the future.").OnField("created_at")
	ErrImportTimeTooEarly = fault.New(fault.Invalid, "import_time_too_early",
		"The time cannot be before the year 2000.").OnField("created_at")
	ErrImportOutOfOrder = fault.New(fault.Invalid, "import_out_of_order",
		"Send history oldest first: this time is before the conversation's latest entry.").OnField("created_at")
	ErrImportClosedBeforeStart = fault.New(fault.Invalid, "import_closed_before_start",
		"The conversation cannot close before its first message.").OnField("closed_at")
)
