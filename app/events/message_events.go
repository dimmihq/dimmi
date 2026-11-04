/*
|--------------------------------------------------------------------------
| Message Events
|--------------------------------------------------------------------------
|
| An entry joined a conversation's timeline. It is dispatched in the
| transaction that wrote it, so listeners that bill or notify for it see
| it only when it is kept.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
MessageCreated says an entry joined a conversation's timeline: a
customer message, a reply or a note. Substantive entries count toward
the conversation's activity; notes do not.
*/
type MessageCreated struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	MessageID      ids.UUID
	Type           string
	AuthorType     string
	AuthorID       ids.UUID
	Substantive    bool
}
