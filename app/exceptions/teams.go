/*
|--------------------------------------------------------------------------
| Team Errors
|--------------------------------------------------------------------------
|
| What can go wrong while changing a workspace's teams. Two live teams
| of one workspace never share a name, whatever its letter case or
| Unicode form, so people always know which team they route to.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Team failures.

ErrTeamNameTaken refuses a name another live team of the workspace
already uses. ErrReassignToInvalid refuses to hand a deleted team's
conversations to anything but another live team or an active member of
the workspace, or to both at once.
*/
var (
	ErrTeamNameTaken = fault.New(fault.Invalid, "team_name_taken",
		"Another team in this workspace already uses this name.").OnField("name")
	ErrReassignToInvalid = fault.New(fault.Invalid, "reassign_to_invalid",
		"Choose one other live team or one active member of this workspace.").OnField("reassign_to")
)
