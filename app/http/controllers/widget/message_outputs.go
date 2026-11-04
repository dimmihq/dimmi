/*
|--------------------------------------------------------------------------
| Widget Message Outputs
|--------------------------------------------------------------------------
|
| The shapes a visitor's messenger receives back about messages: a page of
| them, or one just sent with where it landed.
|
*/

package widget

import (
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
PostedMessageMeta says where a sent message landed.
*/
type PostedMessageMeta struct {
	ConversationStatus string    `json:"conversation_status"`
	Reopened           bool      `json:"reopened"`
	RedirectedFrom     *string   `json:"redirected_from"`
	ServerTime         time.Time `json:"server_time"`
}

/*
PostedMessageOutput is a sent message: 201, or 200 for a repeat.
*/
type PostedMessageOutput struct {
	Status int
	Body   struct {
		Data resources.WidgetMessage `json:"data"`
		Meta PostedMessageMeta       `json:"meta"`
	}
}

/*
MessagesOutput is a page of a conversation's messages.
*/
type MessagesOutput struct {
	Body fhttp.Collection[resources.WidgetMessage]
}
