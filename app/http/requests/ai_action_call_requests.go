/*
|--------------------------------------------------------------------------
| Agent Action Call Requests
|--------------------------------------------------------------------------
|
| The calls the agent made: the team's list and decisions, and the
| visitor's confirmation in the messenger.
|
*/

package requests

/*
AIActionCallsRequest filters and pages the call list.
*/
type AIActionCallsRequest struct {
	Conversation string `query:"conversation_id"`
	Action       string `query:"action_id"`
	State        string `query:"state"`
	Cursor       string `query:"cursor"`
}

/*
AIActionCallRequest names a call in the path.
*/
type AIActionCallRequest struct {
	Call string `path:"call"`
}

/*
AIUpdateActionCallRequest is the input of PATCH /ai/action-calls/{call}:
a teammate's approval or decline, the internal reason, and the message
the agent passes on to the customer after a decline.
*/
type AIUpdateActionCallRequest struct {
	Call string `path:"call"`
	Body struct {
		Status          string `json:"status"                     validate:"required,oneof=approved declined"`
		Reason          string `json:"reason,omitempty"           validate:"omitempty,max=500"`
		CustomerMessage string `json:"customer_message,omitempty" validate:"omitempty,max=500"`
	}
}

/*
WidgetActionCallRequest names a call of the visitor's conversation.
*/
type WidgetActionCallRequest struct {
	Conversation string `path:"conversation"`
	Call         string `path:"call"`
}

/*
UpdateWidgetActionCallRequest is the visitor's confirm or cancel.
*/
type UpdateWidgetActionCallRequest struct {
	Conversation string `path:"conversation"`
	Call         string `path:"call"`
	Body         struct {
		Status string `json:"status" validate:"required,oneof=confirmed canceled"`
	}
}
