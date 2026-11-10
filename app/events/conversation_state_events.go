/*
|--------------------------------------------------------------------------
| Conversation State Events
|--------------------------------------------------------------------------
|
| A conversation moves between people and states: assigned to a member
| or team, closed, reopened, snoozed until later and woken again. The
| reason travels with a reopen or a wake-up, so reports can tell a
| customer writing back from a teammate changing their mind.
|
*/

package events

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
ConversationAssigned is dispatched when the assigned member or team
changes; a nil To is an unassignment. Reason says why the system moved
it, such as arrival or away_reassign, and is empty for a person's move.
*/
type ConversationAssigned struct {
	Base
	ConversationRef

	FromMemberID *ids.UUID
	ToMemberID   *ids.UUID
	FromTeamID   *ids.UUID
	ToTeamID     *ids.UUID
	Reason       string
}

/*
EventName names the event.
*/
func (ConversationAssigned) EventName() string { return "conversation.assigned" }

/*
ConversationClosed is dispatched when a conversation is closed. Reason
is empty when a teammate closed it, else why it closed, such as
contact_solved.
*/
type ConversationClosed struct {
	Base
	ConversationRef

	Reason string
}

/*
EventName names the event.
*/
func (ConversationClosed) EventName() string { return "conversation.closed" }

/*
ConversationReopened is dispatched when a closed conversation opens
again. Reason is customer_message, manual or reply.
*/
type ConversationReopened struct {
	Base
	ConversationRef

	Reason string
}

/*
EventName names the event.
*/
func (ConversationReopened) EventName() string { return "conversation.reopened" }

/*
ConversationSnoozed is dispatched when a conversation is snoozed until
a time.
*/
type ConversationSnoozed struct {
	Base
	ConversationRef

	Until time.Time
}

/*
EventName names the event.
*/
func (ConversationSnoozed) EventName() string { return "conversation.snoozed" }

/*
ConversationUnsnoozed is dispatched when a snooze ends. Reason is
expired, customer_message or manual.
*/
type ConversationUnsnoozed struct {
	Base
	ConversationRef

	Reason string
}

/*
EventName names the event.
*/
func (ConversationUnsnoozed) EventName() string { return "conversation.unsnoozed" }
