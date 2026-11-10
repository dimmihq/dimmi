/*
|--------------------------------------------------------------------------
| Broadcast Events
|--------------------------------------------------------------------------
|
| A broadcast's report fills in while it sends: every new count and every
| change of status reaches the open screens of the workspace, which read
| the numbers straight from the event.
|
*/

package events

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
BroadcastUpdated is dispatched in the transaction that counted a
broadcast again or moved its status. PauseReason says why it paused,
empty otherwise, and Counts holds its latest tally.
*/
type BroadcastUpdated struct {
	Base
	BroadcastID ids.UUID
	Status      string
	PauseReason string
	Counts      json.RawMessage
}

/*
EventName names the event.
*/
func (BroadcastUpdated) EventName() string {
	return "broadcast.updated"
}

/*
BroadcastRecipientChanged is dispatched in the transaction that changed
what happened to a broadcast's recipient outside the broadcast's own
jobs, such as a bounce or a reply, so the broadcast counts again.
*/
type BroadcastRecipientChanged struct {
	Base
	BroadcastID ids.UUID
	RecipientID ids.UUID
}

/*
EventName names the event.
*/
func (BroadcastRecipientChanged) EventName() string {
	return "broadcast.recipient_changed"
}
