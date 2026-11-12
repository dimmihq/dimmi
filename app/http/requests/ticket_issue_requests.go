/*
|--------------------------------------------------------------------------
| Ticket Issue Requests
|--------------------------------------------------------------------------
|
| The inputs of linked issues: creating or linking one on a ticket,
| unlinking one, the lists a connected tool offers to pick from, a
| search of its issues, its sync settings, connecting a tool with an
| API key, the drafts closing the loop, and the workspace's switches.
|
*/

package requests

/*
TicketIssuesRequest names a ticket in the path.
*/
type TicketIssuesRequest struct {
	Ticket string `path:"ticket"`
}

/*
StoreTicketIssueRequest is the input of POST /tickets/{ticket}/issues:
create an issue (installation, team, project, title) or link one (url
or key).
*/
type StoreTicketIssueRequest struct {
	Ticket string `path:"ticket"`
	Body   struct {
		InstallationID *string `json:"installation_id,omitempty" maxLength:"64"`
		TeamID         string  `json:"team_id,omitempty"         maxLength:"200"`
		ProjectID      string  `json:"project_id,omitempty"      maxLength:"200"`
		ServiceID      string  `json:"service_id,omitempty"      maxLength:"200"`
		SeverityID     string  `json:"severity_id,omitempty"     maxLength:"200"`
		Title          string  `json:"title,omitempty"           maxLength:"200"`
		URL            string  `json:"url,omitempty"             maxLength:"2048"`
		Key            string  `json:"key,omitempty"             maxLength:"200"`
	}
}

/*
TicketIssueRequest names a linked issue in the path.
*/
type TicketIssueRequest struct {
	Issue string `path:"issue"`
}
