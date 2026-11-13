/*
|--------------------------------------------------------------------------
| Ticket Panel Outputs
|--------------------------------------------------------------------------
|
| What a conversation's ticket panel reads: the conversation's own
| ticket and trackers, and the AI's field values.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
ConversationTicketsOutput is a conversation's own ticket and the
trackers it joined.
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
