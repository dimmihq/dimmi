/*
|--------------------------------------------------------------------------
| Team Actions
|--------------------------------------------------------------------------
|
| The actions behind a workspace's teams: listing them, creating,
| renaming, deleting and purging them, and choosing their members.
|
*/

package providers

import (
	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/teams"
)

/*
bootTeams builds the actions on teams and their members.
*/
func bootTeams(a *app.App) {
	acts := a.Actions
	acts.ListTeams = &teams.ListTeams{DB: a.DB}
	acts.ShowTeam = &teams.ShowTeam{DB: a.DB}
	acts.ListMemberTeams = &teams.ListMemberTeams{DB: a.DB}
	acts.CreateTeam = &teams.CreateTeam{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.UpdateTeam = &teams.UpdateTeam{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.DeleteTeam = &teams.DeleteTeam{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.PurgeTeam = &teams.PurgeTeam{DB: a.DB}
	acts.AddTeamMembers = &teams.AddTeamMembers{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.RemoveTeamMember = &teams.RemoveTeamMember{DB: a.DB, Events: a.Events}
}
