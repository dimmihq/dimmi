/*
|--------------------------------------------------------------------------
| Conversation Rating Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on conversation ratings: the workspace's
| settings for asking, the customer's answer from the messenger or from
| the page an email links to. A comment is bounded like a short note.
|
*/

package requests

/*
UpdateConversationRatingSettingsRequest is the input of PATCH
/conversation-rating-settings; a field left out keeps its value.
*/
type UpdateConversationRatingSettingsRequest struct {
	Body struct {
		Enabled            *bool `json:"enabled,omitempty"`
		DelayMinutes       *int  `json:"delay_minutes,omitempty"        validate:"omitempty,oneof=0 10 60"`
		IncludeAgentClosed *bool `json:"include_agent_closed,omitempty"`
		Messenger          *bool `json:"messenger,omitempty"`
		Email              *bool `json:"email,omitempty"`
	}
}

/*
ConversationRatingTokenRequest names the signed link of a rating page.
*/
type ConversationRatingTokenRequest struct {
	Token string `path:"token"`
}

/*
RatingAnswerBody is a customer's answer: a score from 1 to 5, a
comment, or both.
*/
type RatingAnswerBody struct {
	Score   *int    `json:"score,omitempty"   minimum:"1" maximum:"5"`
	Comment *string `json:"comment,omitempty" maxLength:"2000"`
}

/*
UpdateConversationRatingByTokenRequest is the input of PATCH
/conversation-ratings/{token}.
*/
type UpdateConversationRatingByTokenRequest struct {
	Token string `path:"token"`
	Body  RatingAnswerBody
}

/*
UpdateWidgetConversationRatingRequest is the input of PATCH
/widget/v1/conversations/{conversation}/rating.
*/
type UpdateWidgetConversationRatingRequest struct {
	Conversation string `path:"conversation"`
	Body         RatingAnswerBody
}
