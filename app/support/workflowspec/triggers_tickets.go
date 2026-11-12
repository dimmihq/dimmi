/*
|--------------------------------------------------------------------------
| Ticket Triggers
|--------------------------------------------------------------------------
|
| A customer ticket's changes start workflows on its conversation: the
| ticket made, moved to another state, its fields changed, the customer
| writing on it, linked to other work, its linked issue moving, and the
| ticket entering close the loop. A ticket without a conversation runs
| no workflow. Workflows hear a move, the start and close the loop with
| a message step, so the team decides in a workflow what the customer
| is told; nothing is sent by itself.
|
*/

package workflowspec

/*
The ticket trigger keys.
*/
const (
	TriggerTicketCreated         = "ticket.created"
	TriggerTicketStateChanged    = "ticket.state_changed"
	TriggerTicketFieldChanged    = "ticket.field_changed"
	TriggerTicketCustomerReplied = "ticket.customer_replied"
	TriggerTicketLinked          = "ticket.linked"
	TriggerTicketIssueUpdated    = "ticket.issue_updated"
	TriggerTicketCloseLoop       = "ticket.close_loop"
)

/*
ticketTriggers returns the triggers of a customer ticket's changes.
*/
func ticketTriggers() []Trigger {
	t := func(key, name string, talks ...string) Trigger {
		return Trigger{
			Key: key, Name: name, Group: GroupTicket, Reentry: reentryAny(), Default: ReentryEveryTime,
			Talks: talks, ConversationFields: true,
		}
	}

	return []Trigger{
		t(TriggerTicketCreated, "Ticket is created", KindMessage),
		t(TriggerTicketStateChanged, "Ticket state changes", KindMessage),
		t(TriggerTicketFieldChanged, "Ticket fields change"),
		t(TriggerTicketCustomerReplied, "Customer replies on a ticket"),
		t(TriggerTicketLinked, "Ticket is linked"),
		t(TriggerTicketIssueUpdated, "Linked issue changes", KindMessage),
		t(TriggerTicketCloseLoop, "Ticket needs the loop closed", KindMessage),
	}
}

/*
TicketTrigger reports whether the trigger is about a ticket.
*/
func TicketTrigger(key string) bool {
	t, ok := FindTrigger(key)

	return ok && t.Group == GroupTicket
}
