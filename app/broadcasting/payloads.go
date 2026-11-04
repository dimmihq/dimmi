/*
|--------------------------------------------------------------------------
| Broadcast Payloads
|--------------------------------------------------------------------------
|
| A broadcast shows the subject of an event as it is now, read when the
| broadcast goes out, never a copy the event carried: two changes in a
| row then both show the latest state, whichever arrives last. Lists get
| the compact row they render, the open conversation gets the whole
| record, and a timeline gets the message.
|
| The shapes are the API's, so a screen patches what it fetched with
| what it hears without translating. A subject that no longer exists has
| nothing to show and is not broadcast.
|
*/

package broadcasting

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Payloads renders the subjects of broadcasts. Each method reports false
when the subject no longer exists.
*/
type Payloads interface {
	/*
		ConversationRow renders the conversation as lists show it.
	*/
	ConversationRow(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error)

	/*
		Conversation renders the whole conversation.
	*/
	Conversation(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error)

	/*
		Message renders the message.
	*/
	Message(ctx context.Context, workspaceID, id ids.UUID) (any, bool, error)
}

/*
Reference is what a broadcast carries in place of a subject too large to
broadcast; clients fetch the rest.
*/
type Reference struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	UpdatedAt time.Time `json:"updated_at"`
}

/*
Stamped is a broadcast subject that knows when it last changed, which a
reference in its place carries.
*/
type Stamped interface {
	/*
		LastUpdated returns when the subject last changed.
	*/
	LastUpdated() time.Time
}

/*
AuthorRef is who is acting in a broadcast: their type and, for a member
or contact, their public id.
*/
type AuthorRef struct {
	Type string  `json:"type"`
	ID   *string `json:"id"`
}
