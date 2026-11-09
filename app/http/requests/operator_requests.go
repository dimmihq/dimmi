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
StoreOperatorThreadRequest is the input of POST /operator/threads: a
conversation's thread, or a new thread about the whole workspace when
no conversation is named.
*/
type StoreOperatorThreadRequest struct {
	Body struct {
		ConversationID *string `json:"conversation_id,omitempty" validate:"omitempty,max=64"`
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
/conversations/{conversation}/operator/brief.
*/
type BriefConversationRequest struct {
	Conversation string `path:"conversation"`
}
