/*
|--------------------------------------------------------------------------
| Feedback Board Requests
|--------------------------------------------------------------------------
|
| The inputs of a help center's boards: adding one, changing one,
| removing one with where its requests go, and saving their order.
|
*/

package requests

/*
FeedbackBoardBody is a board's settings; fields left out stay as they
are.
*/
type FeedbackBoardBody struct {
	Name             *string `json:"name,omitempty"              maxLength:"60"`
	Slug             *string `json:"slug,omitempty"              maxLength:"60"`
	Description      *string `json:"description,omitempty"       maxLength:"500"`
	Visibility       *string `json:"visibility,omitempty"        enum:"public,private,internal"`
	ApprovalRequired *bool   `json:"approval_required,omitempty"`
}

/*
StoreFeedbackBoardRequest is the input of POST
/help-centers/{help_center}/feedback-boards.
*/
type StoreFeedbackBoardRequest struct {
	HelpCenter string `path:"help_center"`
	Body       FeedbackBoardBody
}

/*
UpdateFeedbackBoardRequest is the input of PATCH /feedback-boards/{board}.
*/
type UpdateFeedbackBoardRequest struct {
	Board string `path:"board"`
	Body  FeedbackBoardBody
}

/*
DestroyFeedbackBoardRequest is the input of DELETE
/feedback-boards/{board}: move_to is the board its requests move to,
the help center's first other board when left out.
*/
type DestroyFeedbackBoardRequest struct {
	Board  string `path:"board"`
	MoveTo string `query:"move_to" maxLength:"64"`
}

/*
ReorderFeedbackBoardsRequest is the input of PATCH
/help-centers/{help_center}/feedback-boards: every board's id once, in
the new order.
*/
type ReorderFeedbackBoardsRequest struct {
	HelpCenter string `path:"help_center"`
	Body       struct {
		Boards []string `json:"boards" maxItems:"50" validate:"required,min=1"`
	}
}
