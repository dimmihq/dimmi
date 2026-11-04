/*
|--------------------------------------------------------------------------
| Widget Conversation Outputs
|--------------------------------------------------------------------------
|
| The shapes a visitor's messenger receives back about its conversations. A
| write that the browser repeats answers 200 with what the first write
| stored, so a retry is told it already happened.
|
*/

package widget

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
ConversationsOutput is a page of the visitor's conversations.
*/
type ConversationsOutput struct {
	Body fhttp.Collection[resources.WidgetConversation]
}

/*
ConversationOutput is one of the visitor's conversations.
*/
type ConversationOutput struct {
	Body fhttp.Resource[resources.WidgetConversation]
}

/*
StartedConversation is a new conversation with its first message.
*/
type StartedConversation struct {
	Data resources.WidgetConversation `json:"data"`
	Meta struct {
		Message resources.WidgetMessage `json:"message"`
	} `json:"meta"`
}

/*
StartedConversationOutput is the answer to starting a conversation: 201,
or 200 when the browser sent the same message before.
*/
type StartedConversationOutput struct {
	Status int
	Body   StartedConversation
}
