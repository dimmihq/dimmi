/*
|--------------------------------------------------------------------------
| Operator Requests
|--------------------------------------------------------------------------
|
| What the API accepts for Operator: a change to its settings, a thread
| to open, a question, and the thread or conversation a request names.
|
*/

package requests

/*
UpdateOperatorSettingsRequest is the input of PATCH /operator/settings.
Fields left out keep their values; policies merge by group.
*/
type UpdateOperatorSettingsRequest struct {
	Body struct {
		Enabled         *bool             `json:"enabled,omitempty"`
		BriefOnOpen     *bool             `json:"brief_on_open,omitempty"`
		TriageOnArrival *bool             `json:"triage_on_arrival,omitempty"`
		Instructions    *string           `json:"instructions,omitempty"      validate:"omitempty,max=8000"`
		Policies        map[string]string `json:"policies,omitempty"`
		Model           *string           `json:"model,omitempty"             validate:"omitempty,max=200"`
	}
}

/*
StoreOperatorThreadRequest is the input of POST /operator/threads: the
latest thread about a conversation, or a new one when fresh is set, or
a new thread about the whole workspace when no conversation is named.
*/
type StoreOperatorThreadRequest struct {
	Body struct {
		ConversationID *string `json:"conversation_id,omitempty" validate:"omitempty,max=64"`
		Fresh          bool    `json:"fresh,omitempty"`
	}
}

/*
OperatorThreadRequest names one thread.
*/
type OperatorThreadRequest struct {
	Thread string `path:"thread"`
}

/*
AskOperatorRequest is the input of POST /operator/threads/{thread}/messages.
*/
type AskOperatorRequest struct {
	Thread string `path:"thread"`
	Body   struct {
		Body string `json:"body" validate:"required,max=8000"`
	}
}

/*
BriefConversationRequest is the input of POST
/conversations/{conversation}/operator/brief: the thread to brief on,
the latest when none is named, and whether to keep the brief on the
conversation as a note.
*/
type BriefConversationRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		ThreadID *string `json:"thread_id,omitempty" validate:"omitempty,max=64"`
		Attach   bool    `json:"attach,omitempty"`
	}
}
