/*
|--------------------------------------------------------------------------
| Saved Reply Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a team keeps the replies and macros it inserts
| instead of typing them again. Each refusal carries a stable code
| programs can branch on.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Saved reply failures.

ErrSavedReplyTitleRequired and ErrSavedReplyBodyRequired refuse a reply
without a title or text. ErrSavedReplyShortcutTaken refuses a shortcut
the owner, or the workspace for a shared reply, already uses.
ErrSavedReplyLimitReached refuses a reply past the most a workspace may
keep. ErrInvalidMacro refuses actions that name an unknown status,
priority, snooze, teammate, team or tag.
*/
var (
	ErrSavedReplyTitleRequired = fault.New(fault.Invalid, "saved_reply_title_required",
		"Give the saved reply a title.").OnField("title")
	ErrSavedReplyBodyRequired = fault.New(fault.Invalid, "saved_reply_body_required",
		"Give the saved reply some text.").OnField("body")
	ErrSavedReplyShortcutTaken = fault.New(fault.Conflict, "saved_reply_shortcut_taken",
		"Another saved reply already uses this shortcut.").OnField("shortcut")
	ErrSavedReplyLimitReached = fault.New(fault.Invalid, "saved_reply_limit_reached",
		"A workspace keeps at most 2,000 saved replies.")
	ErrInvalidMacro = fault.New(fault.Invalid, "invalid_macro",
		"The actions name a status, snooze, priority, teammate, team or tag the workspace does not have.").
		OnField("actions")
)
