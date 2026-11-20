/*
|--------------------------------------------------------------------------
| Issue Picker Requests
|--------------------------------------------------------------------------
|
| The inputs of a connected tool's lists and of a search of its issues.
|
*/

package requests

/*
IssueTeamsRequest names an installation in the path. Refresh pulls
the teams from the tool again instead of the copy kept for a day.
*/
type IssueTeamsRequest struct {
	Installation string `path:"installation"`
	Refresh      bool   `query:"refresh"`
}

/*
IssueOptionsRequest asks for one list of a tool: teams, projects,
states, severities, services, users, fields or environments, within a
parent such as a team. Refresh pulls it from the tool again instead of
the copy kept for a day.
*/
type IssueOptionsRequest struct {
	Installation string `path:"installation"`
	List         string `query:"list"         enum:"teams,projects,states,severities,services,users,fields,environments"` //nolint:lll // one enum
	Parent       string `query:"parent"       maxLength:"200"`
	Refresh      bool   `query:"refresh"`
}

/*
IssueSearchRequest searches a tool's issues.
*/
type IssueSearchRequest struct {
	Installation string `path:"installation"`
	Q            string `query:"q"            maxLength:"200"`
}
