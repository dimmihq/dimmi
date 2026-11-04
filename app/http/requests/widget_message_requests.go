/*
|--------------------------------------------------------------------------
| Widget Message Requests
|--------------------------------------------------------------------------
|
| What a visitor's messenger sends about the messages of one conversation:
| which page to read, how far they read, and whether they are typing.
|
*/

package requests

/*
IndexWidgetMessagesRequest is the input of GET .../messages: before scrolls
back from a cursor, after catches up from a message the visitor holds.
*/
type IndexWidgetMessagesRequest struct {
	Conversation string `path:"conversation"`
	Before       string `query:"before"`
	After        string `query:"after"`
	Limit        int    `query:"limit"        maximum:"100" minimum:"0"`
}

/*
UpdateWidgetReadStateRequest is the input of PATCH .../read-state.
*/
type UpdateWidgetReadStateRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		_                 struct{} `additionalProperties:"true"`
		LastReadMessageID string   `json:"last_read_message_id" maxLength:"64"`
	}
}

/*
StoreWidgetTypingRequest is the input of POST .../typing-indicators.
*/
type StoreWidgetTypingRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		_     struct{} `additionalProperties:"true"`
		State string   `enum:"started,stopped" json:"state"`
	}
}
