/*
|--------------------------------------------------------------------------
| Conversation State Resources
|--------------------------------------------------------------------------
|
| Where a conversation stands for the reader: the conversations around
| it in the list they move through, and how far they have read it.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ConversationNeighbors are the conversations before and after one in a
list, null at either end.
*/
type ConversationNeighbors struct {
	Object string           `json:"object"`
	Prev   *ConversationRef `json:"prev"`
	Next   *ConversationRef `json:"next"`
}

/*
NewConversationNeighbors shapes the neighbors.
*/
func NewConversationNeighbors(n conversations.Neighbors) ConversationNeighbors {
	out := ConversationNeighbors{Object: "conversation_neighbors"}
	if n.Prev != nil {
		out.Prev = new(NewConversationRef(*n.Prev))
	}

	if n.Next != nil {
		out.Next = new(NewConversationRef(*n.Next))
	}

	return out
}

/*
ConversationReadState is how far the caller has read a conversation.
*/
type ConversationReadState struct {
	Object            string     `json:"object"`
	ConversationID    string     `json:"conversation_id"`
	LastReadMessageID *string    `json:"last_read_message_id"`
	LastReadAt        *time.Time `json:"last_read_at"`
	Unread            bool       `json:"unread"`
}

/*
NewConversationReadState shapes the read state.
*/
func NewConversationReadState(s conversations.ReadState) ConversationReadState {
	out := ConversationReadState{
		Object: "conversation_read_state", ConversationID: models.PublicID(models.PrefixConversation, s.ConversationID),
		LastReadAt: s.LastReadAt, Unread: s.Unread,
	}

	if s.LastReadMessageID != nil {
		out.LastReadMessageID = new(models.PublicID(models.PrefixMessage, *s.LastReadMessageID))
	}

	return out
}
