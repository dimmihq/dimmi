/*
|--------------------------------------------------------------------------
| Feedback Status Requests
|--------------------------------------------------------------------------
|
| The inputs of the workspace's statuses: their order, a new status, a
| change to one, and removing one with where its requests go. Whether
| an order names each status exactly once is the action's concern.
|
*/

package requests

/*
ReorderFeedbackStatusesRequest is the statuses' keys in their new order.
*/
type ReorderFeedbackStatusesRequest struct {
	Body struct {
		Statuses []string `json:"statuses" maxItems:"50" validate:"required,min=1"`
	}
}

/*
FeedbackStatusBody is a status's settings; fields left out stay as they
are.
*/
type FeedbackStatusBody struct {
	Name     *string `json:"name,omitempty"     maxLength:"40"`
	Color    *string `json:"color,omitempty"    maxLength:"7"`
	Category *string `json:"category,omitempty" enum:"open,planned,in_progress,done,closed"`
	Roadmap  *bool   `json:"roadmap,omitempty"`
}

/*
StoreFeedbackStatusRequest is the input of POST /feedback-statuses.
*/
type StoreFeedbackStatusRequest struct {
	Body FeedbackStatusBody
}

/*
UpdateFeedbackStatusRequest is the input of PATCH
/feedback-statuses/{status}, named by its key.
*/
type UpdateFeedbackStatusRequest struct {
	Status string `path:"status"`
	Body   FeedbackStatusBody
}

/*
DestroyFeedbackStatusRequest is the input of DELETE
/feedback-statuses/{status}: move_to is the key of the status its
requests move to.
*/
type DestroyFeedbackStatusRequest struct {
	Status string `path:"status"`
	MoveTo string `query:"move_to" maxLength:"40"`
}
