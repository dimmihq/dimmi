/*
|--------------------------------------------------------------------------
| Agent Action Call Events
|--------------------------------------------------------------------------
|
| Dispatched as a call the agent requested moves along: proposed,
| confirmed by the customer, approved or declined by a teammate, run,
| expired. Open screens refresh the conversation; a call waiting for a
| teammate also raises its own event, which is what notifications hear.
|
*/

package events

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
AiActionCallUpdated is dispatched on every state change of a live call:
CallID names it and State says where it is now.
*/
type AiActionCallUpdated struct {
	Base
	ConversationRef

	CallID ids.UUID
	State  string
}

/*
EventName names the event.
*/
func (AiActionCallUpdated) EventName() string { return "ai.action_call_updated" }

/*
AiActionCallApprovalRequested is dispatched when a call waits for a
teammate's approval: Approval says who may decide (teammate or admin),
Summary what would run, and ExpiresAt when the request lapses.
*/
type AiActionCallApprovalRequested struct {
	Base
	ConversationRef

	CallID    ids.UUID
	Title     string
	Summary   string
	Approval  string
	ExpiresAt time.Time
}

/*
EventName names the event.
*/
func (AiActionCallApprovalRequested) EventName() string { return "ai.action_call_approval_requested" }
