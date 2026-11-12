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

/*
BroadcastSent is dispatched in the transaction that marked a broadcast
sent, once every recipient was handed out.
*/
type BroadcastSent struct {
	Base
	BroadcastID ids.UUID
}

/*
EventName names the event.
*/
func (BroadcastSent) EventName() string {
	return "broadcast.sent"
}

/*
BroadcastPaused is dispatched in the transaction that paused a sending
broadcast. Reason says why: a teammate paused it, or too many bounces
or complaints.
*/
type BroadcastPaused struct {
	Base
	BroadcastID ids.UUID
	Reason      string
}

/*
EventName names the event.
*/
func (BroadcastPaused) EventName() string {
	return "broadcast.paused"
}
