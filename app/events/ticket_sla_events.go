/*
|--------------------------------------------------------------------------
| Ticket Sla Events
|--------------------------------------------------------------------------
|
| Service level clocks warned or breached on a ticket or a conversation,
| and tickets moved in and out of the trash.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TicketSLAWarning says a ticket's service level target is about to be
missed; Changes holds the target and when it is due.
*/
type TicketSLAWarning struct{ TicketChanged }

/*
TicketSLABreached says a ticket missed a service level target.
*/
type TicketSLABreached struct{ TicketChanged }

/*
TicketDeleted says a ticket moved to the trash.
*/
type TicketDeleted struct{ TicketChanged }

/*
TicketRestored says a ticket came back from the trash.
*/
type TicketRestored struct{ TicketChanged }

/*
SLAClockAlerted says a service level clock on a conversation warned or
breached; Breached tells which. Workflows on the conversation hear it.
*/
type SLAClockAlerted struct {
	Base
	ConversationRef
	ClockID  ids.UUID
	PolicyID ids.UUID
	Target   string
	Breached bool
}

/*
EventName names the event.
*/
func (e SLAClockAlerted) EventName() string {
	if e.Breached {
		return "sla.breached"
	}

	return "sla.warning"
}
