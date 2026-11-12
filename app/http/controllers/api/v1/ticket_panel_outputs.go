/*
|--------------------------------------------------------------------------
| Ticket Panel Outputs
|--------------------------------------------------------------------------
|
| What a conversation's ticket panel and an incident's public log read:
| the conversation's own ticket and trackers, the AI's field values, and
| the incident's public posts.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
ConversationTicketsOutput is a conversation's own ticket and the
trackers and incidents it joined.
*/
type ConversationTicketsOutput struct {
	Body struct {
		Data struct {
			Ticket   *resources.Ticket  `json:"ticket"`
			Trackers []resources.Ticket `json:"trackers"`
		} `json:"data"`
	}
}

/*
TicketSuggestionsOutput is the AI's values for a type's fields.
*/
type TicketSuggestionsOutput struct {
	Body fhttp.Resource[map[string]any]
}

/*
TicketPublicUpdatesOutput is an incident's public log.
*/
type TicketPublicUpdatesOutput struct {
	Body fhttp.Resource[[]resources.TicketPublicUpdate]
}

/*
TicketPublicUpdateOutput is one post.
*/
type TicketPublicUpdateOutput struct {
	Body fhttp.Resource[resources.TicketPublicUpdate]
}
