/*
|--------------------------------------------------------------------------
| Inbox Setting Failures
|--------------------------------------------------------------------------
|
| Refusals of a change to an inbox's settings or of its deletion. Each
| names the setting it concerns, so the settings page shows it beside the
| right control. The default inbox is where everything without a home
| arrives, so it is never deleted, and a non-empty inbox only goes once
| its conversations have somewhere to go.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Inbox setting failures.

ErrInboxSettingInvalid refuses a setting outside what it accepts; blame
the setting with OnField. ErrInboxTeamNotFound refuses a team the
workspace does not have. ErrOverflowIsDefaultTeam refuses an overflow
team that is the inbox's own default team. ErrDefaultInboxUndeletable
refuses deleting the default inbox. ErrMoveToRequired refuses deleting
an inbox with conversations without saying where they go, and
ErrMoveToInvalid a target that is the inbox itself or not a live inbox.
ErrInboxAccessInvalid refuses an access other than everyone or
restricted, and ErrInboxAccessMemberNotFound a teammate who is not an
active member of the workspace.
*/
var (
	ErrInboxSettingInvalid = fault.New(fault.Invalid, "invalid_inbox_setting",
		"This setting is not valid.")
	ErrInboxTeamNotFound = fault.New(fault.Invalid, "team_not_found",
		"This team does not exist in this workspace.")
	ErrOverflowIsDefaultTeam = fault.New(fault.Invalid, "overflow_team_is_default",
		"The overflow team must differ from the inbox's default team.").OnField("settings.assignment.overflow_team_id")
	ErrDefaultInboxUndeletable = fault.New(fault.Invalid, "default_inbox",
		"The default inbox cannot be deleted.")
	ErrMoveToRequired = fault.New(fault.Invalid, "move_to_required",
		"Choose an inbox for this inbox's conversations.").OnField("move_to")
	ErrMoveToInvalid = fault.New(fault.Invalid, "move_to_invalid",
		"Conversations can only move to another inbox of this workspace.").OnField("move_to")
	ErrInboxAccessInvalid = fault.New(fault.Invalid, "invalid_inbox_access",
		"Choose everyone or restricted.").OnField("access")
	ErrInboxAccessMemberNotFound = fault.New(fault.Invalid, "member_not_found",
		"This teammate is not an active member of this workspace.").OnField("members")
)
