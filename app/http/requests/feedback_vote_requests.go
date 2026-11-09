/*
|--------------------------------------------------------------------------
| Feature Request Vote and Comment Requests
|--------------------------------------------------------------------------
|
| The inputs of voting for a customer, taking a vote back, and the
| team's comments and internal notes.
|
*/

package requests

/*
StoreFeedbackVoteRequest is the input of POST
/feedback-posts/{post}/votes: a vote the team casts for a customer.
*/
type StoreFeedbackVoteRequest struct {
	Post string `path:"post"`
	Body struct {
		ContactID string `json:"contact_id" maxLength:"64" minLength:"1"`
		Priority  string `json:"priority,omitempty" enum:"nice,important,must,"`
	}
}

/*
DestroyFeedbackVoteRequest is the input of DELETE
/feedback-posts/{post}/votes/{contact}.
*/
type DestroyFeedbackVoteRequest struct {
	Post    string `path:"post"`
	Contact string `path:"contact"`
}

/*
StoreFeedbackCommentRequest is the input of POST
/feedback-posts/{post}/comments: a reply, or an internal note.
*/
type StoreFeedbackCommentRequest struct {
	Post string `path:"post"`
	Body struct {
		Body     string `json:"body"                minLength:"1" maxLength:"5000"`
		ParentID string `json:"parent_id,omitempty" maxLength:"64"`
		Internal bool   `json:"internal,omitempty"`
	}
}

/*
FeedbackCommentRequest names a comment in the path.
*/
type FeedbackCommentRequest struct {
	Comment string `path:"comment"`
}
