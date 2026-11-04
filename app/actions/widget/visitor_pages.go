/*
|--------------------------------------------------------------------------
| Visitor List Pages
|--------------------------------------------------------------------------
|
| How a visitor's lists are read a page at a time. A page starts after the
| last item the previous one held, by time and id, so items that arrive while
| the visitor scrolls never shift what comes next.
|
*/

package widget

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Cursor is where a page of a visitor's list starts: the time and id of the
last item the previous page held.
*/
type Cursor struct {
	At time.Time
	ID ids.UUID
}

/*
ConversationQuery is a page of a visitor's conversations to read.
*/
type ConversationQuery struct {
	Auth     Authenticated
	Statuses []string
	Limit    int
	After    *Cursor
}

/*
ConversationPage is a page of a visitor's conversations, and where the next
page starts.
*/
type ConversationPage struct {
	Items []VisibleConversation
	Next  *Cursor
}

/*
MessageQuery is a page of one conversation's messages to read: older than
Before, or newer than After, which catches up from a message the visitor
already holds.
*/
type MessageQuery struct {
	Auth         Authenticated
	Conversation ids.UUID
	Limit        int
	Before       *Cursor
	After        *Cursor
}

/*
MessagePage is a page of messages, oldest first, and whether older ones
remain.
*/
type MessagePage struct {
	Items   []VisibleMessage
	HasMore bool
}
