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
ErrTeamNameTaken refuses a name another live team of the workspace
already uses.
*/
var ErrTeamNameTaken = fault.New(fault.Invalid, "team_name_taken",
	"Another team in this workspace already uses this name.").OnField("name")
