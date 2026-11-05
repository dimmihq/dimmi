/*
|--------------------------------------------------------------------------
| Inbox Access Requests
|--------------------------------------------------------------------------
|
| Input of the endpoint that says who may open an inbox. The list arrives
| whole and replaces the inbox's own.
|
*/

package requests

/*
UpdateInboxAccessRequest is the input of PUT /inboxes/{inbox}/access:
everyone, or restricted to the teams and teammates listed, which replace
the inbox's whole list.
*/
type UpdateInboxAccessRequest struct {
	Inbox string `path:"inbox"`
	Body  struct {
		Mode      string   `json:"mode" validate:"required,oneof=everyone restricted"`
		TeamIDs   []string `json:"team_ids,omitempty" validate:"max=200"`
		MemberIDs []string `json:"member_ids,omitempty" validate:"max=1000"`
	}
}
