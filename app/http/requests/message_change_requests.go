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

/*
MentionablesRequest is the input of GET
/conversations/{conversation}/mentionables: the conversation and what
the writer typed after the at sign.
*/
type MentionablesRequest struct {
	Conversation string `path:"conversation"`
	Query        string `query:"q" validate:"max=80"`
}

/*
AddReactionRequest is the input of POST
/conversations/{conversation}/messages/{message}/reactions.
*/
type AddReactionRequest struct {
	Conversation string `path:"conversation"`
	Message      string `path:"message"`
	Body         struct {
		Emoji string `json:"emoji" validate:"required,max=32"`
	}
}

/*
RemoveReactionRequest is the input of DELETE
/conversations/{conversation}/messages/{message}/reactions/{emoji},
the emoji URL-encoded.
*/
type RemoveReactionRequest struct {
	Conversation string `path:"conversation"`
	Message      string `path:"message"`
	Emoji        string `path:"emoji"`
}
