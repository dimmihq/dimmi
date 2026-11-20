/*
|--------------------------------------------------------------------------
| Ticket Activity Outputs
|--------------------------------------------------------------------------
|
| What the tracker endpoints answer: a page of a tracker's conversations
| with who it reaches.
|
*/

package v1

import "github.com/dimmihq/dimmi/app/http/resources"

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
