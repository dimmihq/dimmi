/*
|--------------------------------------------------------------------------
| Tickets
|--------------------------------------------------------------------------
|
| The tickets action is built once with the services it uses. Every
| path that changes a ticket, a teammate in the inbox, the API, a
| workflow, the AI agent, MCP or the customer, goes through the same
| writer, so the rules hold whoever acts.
|
*/

package tickets

import (
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
)

/*
Failures of working tickets.
*/
var (
	ErrTicketNotFound = fault.New(fault.NotFound, "ticket_not_found", "No ticket of this workspace has this id.")
	ErrTypeNotFound   = fault.New(fault.NotFound, "ticket_type_not_found",
		"No ticket type of this workspace has this id.")
	ErrStateNotFound = fault.New(fault.NotFound, "ticket_state_not_found",
		"No ticket state of this workspace has this id.")
	ErrFieldNotFound = fault.New(fault.NotFound, "ticket_field_not_found",
		"No ticket field of this workspace has this id.")
	ErrLinkNotFound = fault.New(fault.NotFound, "ticket_link_not_found", "No ticket link has this id.")
	ErrVersionClash = fault.New(fault.Conflict, "version_conflict",
		"The ticket changed since this copy was read. Read it again and retry.")
	ErrMissingFields = fault.New(fault.Invalid, "required_fields_missing",
		"Some required fields have no value.").OnField("fields")
	ErrStateNotAllowed = fault.New(fault.Invalid, "state_not_allowed",
		"The ticket's type does not use this state.").OnField("state_id")
	ErrTypeArchived = fault.New(fault.Invalid, "ticket_type_archived",
		"New tickets cannot use an archived type.").OnField("type_id")
	ErrKindMismatch = fault.New(fault.Invalid, "ticket_kind_mismatch",
		"A ticket can only move to a type of the same kind.").OnField("type_id")
	ErrTitleInvalid    = fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 200 characters.").OnField("title")
	ErrContactRequired = fault.New(fault.Invalid, "contact_required",
		"A customer ticket needs a contact or a conversation.").OnField("contact_id")
	ErrAlreadyTicket = fault.New(fault.Conflict, "conversation_already_ticket",
		"This conversation is already a ticket.").OnField("conversation_id")
	ErrFieldInvalid = fault.New(fault.Invalid, "field_value_invalid",
		"A field value does not fit the field's type.").OnField("fields")
	ErrLinkInvalid = fault.New(fault.Invalid, "ticket_link_invalid",
		"This link does not fit the tickets it joins.").OnField("kind")
	ErrNotTracker = fault.New(fault.Invalid, "ticket_not_tracker",
		"Only trackers and incidents message their linked customers.")
)

/*
Tickets works tickets. Starter creates the conversation of a
customer ticket made with only a contact, and Delivery sends messages
to customers on their own channel.
*/
type Tickets struct {
	DB       *database.DB
	Clock    clock.Clock
	Events   *events.Dispatcher
	Queue    *queue.Queue
	Starter  *conversations.CreateConversation
	Delivery conversations.Delivery
	Filler   Filler
}

/*
services returns what a conversation writer needs from the action.
*/
func (t *Tickets) services() conversations.Services {
	return conversations.Services{Events: t.Events, Clock: t.Clock}
}
