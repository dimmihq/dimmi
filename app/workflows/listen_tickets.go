/*
|--------------------------------------------------------------------------
| Ticket And SLA Triggers
|--------------------------------------------------------------------------
|
| A ticket's changes start workflows on the conversations they concern:
| a customer ticket's own conversation, and for a tracker or back-office
| work, the conversations of the customers linked to it, so an issue
| that ships reaches every one of them. A move into a close-the-loop
| state starts the close the loop trigger beside the state change. A
| ticket with no conversation behind it starts nothing. Service level
| alarms start the SLA triggers of their conversation. Changes the
| agent made, and a workflow's own, never start it again.
|
*/

package workflows

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
maxTicketFanOut bounds how many conversations one ticket change reaches.
*/
const maxTicketFanOut = 1000

/*
listenTickets wires the ticket and SLA triggers.
*/
func (e *Engine) listenTickets(d *events.Dispatcher) {
	events.Listen(d, events.InTx, e.onTicket)
	events.ListenQueued(d, func(ev appevents.SLAClockAlerted) queue.Args {
		return queuedChange(ev.Base, ev.ConversationID, ev.EventName(), map[string]any{"sla.target": ev.Target})
	})
}

/*
ticketTrigger maps a ticket event to its trigger and the data its
conditions read, false for events that start nothing.
*/
func ticketTrigger(ev appevents.TicketEvent) (string, map[string]any, bool) {
	c := ev.Ticket().Changes

	switch ev.EventName() {
	case "ticket.created":
		return workflowspec.TriggerTicketCreated, map[string]any{}, true
	case "ticket.state_changed":
		return workflowspec.TriggerTicketStateChanged, map[string]any{
			"trigger.from_group": c["group"].From, "trigger.to_group": c["group"].To,
		}, true
	case "ticket.updated":
		keys := fieldKeys(c)

		return workflowspec.TriggerTicketFieldChanged, map[string]any{"trigger.fields": keys}, len(keys) > 0
	case "ticket.customer_replied":
		return workflowspec.TriggerTicketCustomerReplied, map[string]any{"message_id": fmt.Sprint(c["message_id"].To)},
			true
	case "ticket.linked":
		link, _ := c["link"].To.(map[string]any)

		return workflowspec.TriggerTicketLinked, map[string]any{"trigger.link_kind": link["kind"]}, true
	case "ticket.issue_updated":
		group := c["status_group"].To
		if group == nil {
			group = c["issue_status_group"].To
		}

		return workflowspec.TriggerTicketIssueUpdated, map[string]any{"trigger.issue_status_group": group}, true
	}

	return "", nil, false
}

/*
fieldKeys returns the field keys a change names, in order.
*/
func fieldKeys(c map[string]appevents.TicketChange) []string {
	var keys []string
	for k := range maps.Keys(c) {
		if key, ok := strings.CutPrefix(k, "fields."); ok {
			keys = append(keys, key)
		}
	}

	slices.Sort(keys)

	return keys
}

/*
ticketChange is one ticket event on its way to the workflows: the
trigger, its data, and the change's header.
*/
type ticketChange struct {
	trigger string
	data    map[string]any
	base    appevents.Base
}

/*
onTicket queues the ticket trigger's matching for each conversation the
change concerns, in the transaction of the change.
*/
func (e *Engine) onTicket(ctx context.Context, ev appevents.TicketEvent) error {
	trigger, data, ok := ticketTrigger(ev)
	changed := ev.Ticket()
	if !ok || changed.Actor.Type == "agent" {
		return nil
	}

	return e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		t, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: changed.WorkspaceID, ID: changed.TicketID})
		if err != nil {
			return nil //nolint:nilerr // a ticket gone starts nothing
		}

		convs, err := e.ticketConversations(ctx, q, t)
		if trigger == workflowspec.TriggerTicketLinked && t.ConversationID == nil {
			convs = e.linkedConversation(ctx, q, t, changed.Changes)
		}

		if err != nil {
			return err
		}

		changes := []ticketChange{{trigger: trigger, data: data, base: changed.Base}}
		if trigger == workflowspec.TriggerTicketStateChanged && t.CloseLoop {
			changes = append(changes, ticketChange{
				trigger: workflowspec.TriggerTicketCloseLoop, data: data, base: changed.Base,
			})
		}

		return e.pushChanges(ctx, tx, convs, changes)
	})
}

/*
pushChanges queues the matching of each change on each conversation.
*/
func (e *Engine) pushChanges(ctx context.Context, tx *database.Tx, convs []ids.UUID, changes []ticketChange) error {
	for _, conv := range convs {
		for _, c := range changes {
			b := c.base
			b.ID = ids.New()

			args := queuedChange(b, conv, c.trigger, c.data)
			if args == nil {
				continue
			}

			if err := e.Queue.PushTx(ctx, tx, args); err != nil {
				return fmt.Errorf("queue ticket trigger: %w", err)
			}
		}
	}

	return nil
}

/*
ticketConversations returns the conversations a ticket's change
concerns: a customer ticket's own, or the conversations linked to a
tracker or back-office work and those of the customer tickets behind it.
*/
func (e *Engine) ticketConversations(ctx context.Context, q *models.Queries, t models.Ticket) ([]ids.UUID, error) {
	if t.ConversationID != nil {
		return []ids.UUID{*t.ConversationID}, nil
	}

	if t.Kind == tickets.KindCustomer {
		return nil, nil
	}

	links, err := q.ListTicketLinks(ctx, models.ListTicketLinksParams{WorkspaceID: t.WorkspaceID, TicketID: t.ID})
	if err != nil {
		return nil, fmt.Errorf("list ticket links: %w", err)
	}

	var out, customers []ids.UUID
	for _, l := range links {
		switch {
		case l.ConversationID != nil:
			out = append(out, *l.ConversationID)
		case l.FromTicketID != t.ID:
			customers = append(customers, l.FromTicketID)
		}
	}

	if len(customers) > 0 {
		rows, err := q.ListTicketsByIDs(ctx, models.ListTicketsByIDsParams{WorkspaceID: t.WorkspaceID, Ids: customers})
		if err != nil {
			return nil, fmt.Errorf("list linked tickets: %w", err)
		}

		for _, r := range rows {
			if r.ConversationID != nil && r.DeletedAt == nil {
				out = append(out, *r.ConversationID)
			}
		}
	}

	slices.SortFunc(out, func(a, b ids.UUID) int { return strings.Compare(a.String(), b.String()) })
	out = slices.Compact(out)

	return out[:min(len(out), maxTicketFanOut)], nil
}

/*
linkedConversation returns the conversation a tracker or back-office
link just joined: the conversation linked, or the customer ticket's.
*/
func (e *Engine) linkedConversation(ctx context.Context, q *models.Queries, t models.Ticket,
	changes map[string]appevents.TicketChange,
) []ids.UUID {
	link, _ := changes["link"].To.(map[string]any)
	if public, ok := link["conversation_id"].(string); ok {
		if id, err := e.IDs.Decode(string(models.PrefixConversation), public); err == nil {
			return []ids.UUID{id}
		}
	}

	public, _ := link["ticket_id"].(string)
	id, err := e.IDs.Decode(string(models.PrefixTicket), public)
	if err != nil {
		return nil
	}

	other, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: t.WorkspaceID, ID: id})
	if err != nil || other.ConversationID == nil {
		return nil
	}

	return []ids.UUID{*other.ConversationID}
}
