/*
|--------------------------------------------------------------------------
| Widget Conversation Requests
|--------------------------------------------------------------------------
|
| What a visitor's messenger sends to start a conversation or write in one.
| Keys it does not know, such as a metadata block a page tries to set, are
| ignored: what the server records about a message is its own to decide.
|
*/

package requests

/*
WidgetMessageBody is a visitor's message. Choice is the button they
tapped, when the message answers a question the messenger drew as
buttons: the workflow then takes that path, whatever the button reads.
*/
type WidgetMessageBody struct {
	_             struct{}        `additionalProperties:"true"`
	ClientID      string          `json:"client_id"                maxLength:"64"`
	Body          string          `json:"body,omitempty"           maxLength:"262144"`
	AttachmentIDs []string        `json:"attachment_ids,omitempty" maxItems:"10"`
	Page          *WidgetPageBody `json:"page,omitempty"`
	Context       map[string]any  `json:"context,omitempty"`
	Website       string          `json:"website,omitempty"        maxLength:"1000"`
	Choice        string          `json:"choice,omitempty"         maxLength:"64"`
}

/*
StoreWidgetConversationRequest is the input of POST /widget/v1/conversations.
*/
type StoreWidgetConversationRequest struct {
	Body WidgetMessageBody
}

/*
StoreWidgetOpenerRequest is the input of POST /widget/v1/openers: the
visitor opened a new chat and said nothing yet, so only where they are
is worth sending.
*/
type StoreWidgetOpenerRequest struct {
	Body struct {
		_       struct{}        `additionalProperties:"true"`
		Page    *WidgetPageBody `json:"page,omitempty"`
		Context map[string]any  `json:"context,omitempty"`
	}
}

/*
ShowWidgetConversationRequest is the input of the routes on one of the
visitor's conversations.
*/
type ShowWidgetConversationRequest struct {
	Conversation string `path:"conversation"`
}

/*
StoreWidgetMessageRequest is the input of POST .../messages.
*/
type StoreWidgetMessageRequest struct {
	Conversation string `path:"conversation"`
	Body         WidgetMessageBody
}
