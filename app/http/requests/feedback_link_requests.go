/*
|--------------------------------------------------------------------------
| Feedback Link Requests
|--------------------------------------------------------------------------
|
| The inputs of linking conversations to feature requests: the requests
| a conversation is linked to, linking one, found or posted from the
| conversation, and taking a link back.
|
*/

package requests

/*
ConversationFeedbackRequest names a conversation in the path.
*/
type ConversationFeedbackRequest struct {
	Conversation string `path:"conversation"`
}

/*
LinkFeedbackPostRequest is the input of POST
/conversations/{conversation}/feedback-posts: the request post_id names,
or a new one with a title, on the help center and board named or the
workspace's default ones. The conversation's customer votes for it with
the priority given.
*/
type LinkFeedbackPostRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		PostID       string  `json:"post_id,omitempty"        maxLength:"64"`
		Title        *string `json:"title,omitempty"          maxLength:"150"`
		BodyMarkdown *string `json:"body_markdown,omitempty"  maxLength:"65536"`
		HelpCenterID string  `json:"help_center_id,omitempty" maxLength:"64"`
		BoardID      string  `json:"board_id,omitempty"       maxLength:"64"`
		Priority     string  `json:"priority,omitempty"       enum:"nice,important,must,"`
	}
}

/*
UnlinkFeedbackPostRequest is the input of DELETE
/conversations/{conversation}/feedback-posts/{post}.
*/
type UnlinkFeedbackPostRequest struct {
	Conversation string `path:"conversation"`
	Post         string `path:"post"`
}
