/*
|--------------------------------------------------------------------------
| Conversation Safety Errors
|--------------------------------------------------------------------------
|
| What a workspace's own redaction rules and link checks can refuse. Each refusal carries
| a stable code programs can branch on, and never echoes the pattern or
| the text it was given.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Redaction failures.

ErrRedactionRuleLimitReached refuses a rule past the ten a workspace may
keep. ErrInvalidRedactionPattern refuses a pattern that is empty, longer
than 256 characters or not a valid expression. ErrRedactionOrder refuses
an order that does not list each of the workspace's rules exactly once.
*/
var (
	ErrRedactionRuleLimitReached = fault.New(fault.Invalid, "redaction_rule_limit_reached",
		"A workspace keeps at most 10 redaction rules.")
	ErrInvalidRedactionPattern = fault.New(fault.Invalid, "invalid_pattern",
		"The pattern must be a valid expression of at most 256 characters.").OnField("pattern")
	ErrRedactionOrder = fault.New(fault.Invalid, "invalid_order",
		"List each of the workspace's rules exactly once.").OnField("order")
)

/*
ErrInvalidLinkDomains refuses an allowed or blocked domain that is not a
hostname a workspace could own, or a list longer than 500 domains.
*/
var ErrInvalidLinkDomains = fault.New(fault.Invalid, "invalid_link_domains",
	"Each domain must be a hostname such as example.com, at most 500 in a list.").OnField("link_safety")

/*
Erasure failures.

ErrErasurePending refuses a second request to erase a customer whose
erasure is already under way. ErrErasureClosed refuses to cancel an
erasure that completed or was canceled already.
*/
var (
	ErrErasurePending = fault.New(fault.Conflict, "erasure_pending",
		"This customer is already being erased.")
	ErrErasureClosed = fault.New(fault.Conflict, "erasure_closed",
		"This erasure has already completed or been canceled.")
)

/*
Export failures.

ErrExportInProgress refuses a second export of a customer while one is
being built. ErrExportNotReady refuses to download an export that is not
built yet, failed, or expired.
*/
var (
	ErrExportInProgress = fault.New(fault.Conflict, "export_in_progress",
		"An export of this customer is already being prepared.")
	ErrExportNotReady = fault.New(fault.Conflict, "export_not_ready",
		"This export is not ready to download.")
)

/*
Data export failures.

ErrExportTooLarge refuses an export covering more rows than its format
allows. ErrExportFormat refuses a format the kind of export does not
come in. ErrInvalidSelection refuses a selection naming something that
is not a row of the kind exported.
*/
var (
	ErrInvalidSelection = fault.New(fault.Invalid, "invalid_selection",
		"The selection names something that cannot be exported.").OnField("selection")
	ErrExportTooLarge = fault.New(fault.Invalid, "export_too_large",
		"This export covers too many rows. Narrow the selection or pick another format.")
	ErrExportFormat = fault.New(fault.Invalid, "invalid_export_format",
		"This export does not come in that format.").OnField("format")
)

/*
ErrInvalidRetention refuses a retention period outside its range: closed
conversations 30 to 3650 days, spam and trash 7 to 365, raw email 0 to
365, data exports 1 to 30.
*/
var ErrInvalidRetention = fault.New(fault.Invalid, "invalid_retention",
	"Choose a period within the allowed range.").OnField("retention")
