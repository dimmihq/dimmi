/*
|--------------------------------------------------------------------------
| Ticket Activity Outputs
|--------------------------------------------------------------------------
|
| What the tracker endpoints answer: a page of a tracker's conversations
| with who it reaches, who a message reaches or holds back, and past
| messages with each recipient's result.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
TrackerConversationsOutput is a page of a tracker's conversations with
who it reaches in all.
*/
type TrackerConversationsOutput struct {
	Body struct {
		Data []resources.TrackerConversation `json:"data"`
		Meta struct {
			NextCursor *string                `json:"next_cursor"`
			Impact     resources.TicketImpact `json:"impact"`
		} `json:"meta"`
	}
}

/*
TicketUpdatePlanOutput is who a message reaches, and the message when
it was sent rather than counted.
*/
type TicketUpdatePlanOutput struct {
	Body struct {
		Data struct {
			Recipients int                     `json:"recipients"`
			Held       []HeldRecipient         `json:"held"`
			Update     *resources.TicketUpdate `json:"update"`
		} `json:"data"`
	}
}

/*
HeldRecipient is a conversation a message holds back and why.
*/
type HeldRecipient struct {
	ConversationID string `json:"conversation_id"`
	Reason         string `json:"reason"`
}

/*
TicketUpdatesOutput is a tracker's messages with each recipient's
result.
*/
type TicketUpdatesOutput struct {
	Body fhttp.Resource[[]resources.TicketUpdate]
}
