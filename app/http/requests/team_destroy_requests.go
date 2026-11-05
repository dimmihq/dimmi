/*
|--------------------------------------------------------------------------
| Team Deletion Requests
|--------------------------------------------------------------------------
|
| Deleting a team says where its conversations go, in the query, so the
| request needs no body: another team, a teammate, or neither.
|
*/

package requests

/*
DestroyTeamRequest is the input of DELETE /teams/{team}: where the
team's conversations go, another team or a member, or neither to leave
them without a team.
*/
type DestroyTeamRequest struct {
	Team             string `path:"team"`
	ReassignToTeam   string `query:"reassign_to_team_id"`
	ReassignToMember string `query:"reassign_to_member_id"`
}
