/*
|--------------------------------------------------------------------------
| Message Change Requests
|--------------------------------------------------------------------------
|
| A teammate edits one of their notes, or removes a message: undoing their
| reply still waiting, or deleting it. Each names the conversation and
| the message.
|
*/

package requests

/*
EditMessageRequest is the input of PATCH
/conversations/{conversation}/messages/{message}.
*/
type EditMessageRequest struct {
	Conversation string `path:"conversation"`
	Message      string `path:"message"`
	Body         struct {
		Body string `json:"body" validate:"required,max=65536"`
	}
}

/*
MessageRequest names one entry of a conversation's timeline.
*/
type MessageRequest struct {
	Conversation string `path:"conversation"`
	Message      string `path:"message"`
}
