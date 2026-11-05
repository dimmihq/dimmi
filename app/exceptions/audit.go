/*
|--------------------------------------------------------------------------
| Audit Log Errors
|--------------------------------------------------------------------------
|
| What can go wrong while exporting a workspace's audit log. An export
| covers a year at most, so one file stays small enough to stream and
| open, and a workspace exports only so often an hour.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
ErrRangeTooLarge refuses an export spanning more than the days it may
cover, and carries max_days. ErrTooManyExports refuses an export once
the workspace exported as often as it may this hour, and carries
retry_after.
*/
var (
	ErrRangeTooLarge = fault.New(fault.Invalid, "range_too_large",
		"An export covers a year at most. Pick a shorter date range.").OnField("filter[created_after]")
	ErrTooManyExports = fault.New(fault.RateLimited, "too_many_exports",
		"This workspace exported its audit log as often as it may this hour. Try again later.")
)
