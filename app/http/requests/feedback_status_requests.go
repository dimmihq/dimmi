/*
|--------------------------------------------------------------------------
| Feedback Status Requests
|--------------------------------------------------------------------------
|
| The input of PATCH /feedback-statuses: every status key once, in the
| order the workspace shows them. Whether the list names each status
| exactly once is the action's concern.
|
*/

package requests

/*
ReorderFeedbackStatusesRequest is the statuses' keys in their new order.
*/
type ReorderFeedbackStatusesRequest struct {
	Body struct {
		Statuses []string `json:"statuses" maxItems:"10" validate:"required,min=1"`
	}
}
