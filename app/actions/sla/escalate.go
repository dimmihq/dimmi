/*
|--------------------------------------------------------------------------
| Alarms And Escalation
|--------------------------------------------------------------------------
|
| A ticket's alarm goes out as a ticket event, with the target, the due
| time, the policy and the clock, and reaches the workflows of the
| ticket's conversation too. A breach raises the priority to the
| policy's, never lowers it, and hands the work to the breach team,
| keeping the teammate who holds it.
|
*/

package sla

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
onTicket announces a ticket's alarm and escalates the ticket and its
conversation on a breach.
*/
func (a *alarm) onTicket(ctx context.Context, id ids.UUID) error {
	w, err := a.s.Tickets.Wrap(ctx, a.writer, a.clock.WorkspaceID)
	if err != nil {
		return err
	}

	t, err := w.Lock(ctx, a.clock.WorkspaceID, id)
	if err != nil || t.DeletedAt != nil {
		return nil //nolint:nilerr // a ticket gone needs no alarm
	}

	if err := a.announceTicket(ctx, t); err != nil {
		return err
	}

	if t.ConversationID != nil {
		if err := a.announceConversation(ctx, *t.ConversationID); err != nil {
			return err
		}
	}

	if !a.breached {
		return nil
	}

	if err := a.escalateTicket(ctx, w, t); err != nil {
		return err
	}

	if t.ConversationID == nil {
		return nil
	}

	conv, err := a.q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: t.WorkspaceID, ID: *t.ConversationID,
	})
	if err != nil {
		return nil //nolint:nilerr // a conversation gone has nothing to escalate
	}

	return a.escalateConversation(ctx, conv)
}

/*
announceTicket sends ticket.sla_warning or ticket.sla_breached.
*/
func (a *alarm) announceTicket(ctx context.Context, t models.Ticket) error {
	c := a.clock
	changed := appevents.TicketChanged{
		Base:      a.writer.Base(t.WorkspaceID),
		TicketRef: appevents.TicketRef{TicketID: t.ID, Kind: t.Kind, ConversationID: t.ConversationID},
		Changes: map[string]appevents.TicketChange{
			"target":        {To: c.Target},
			"due_at":        {To: c.DueAt},
			"sla_policy_id": {To: c.PolicyID},
			"sla_clock_id":  {To: models.PublicID(models.PrefixSLAClock, c.ID)},
		},
	}

	if a.breached {
		return a.writer.Dispatch(ctx, appevents.TicketSLABreached{TicketChanged: changed})
	}

	return a.writer.Dispatch(ctx, appevents.TicketSLAWarning{TicketChanged: changed})
}

/*
announceConversation starts the SLA workflows of the conversation.
*/
func (a *alarm) announceConversation(ctx context.Context, conv ids.UUID) error {
	return a.writer.Dispatch(ctx, appevents.SLAClockAlerted{
		Base: a.writer.Base(a.clock.WorkspaceID), ConversationRef: appevents.ConversationRef{ConversationID: conv},
		ClockID: a.clock.ID, PolicyID: a.clock.PolicyID, Target: a.clock.Target, Breached: a.breached,
	})
}

/*
escalateTicket raises the ticket's priority and moves it to the breach
team.
*/
func (a *alarm) escalateTicket(ctx context.Context, w *tickets.Writer, t models.Ticket) error {
	p := a.policy
	change := tickets.Change{Team: p.BreachTeamID}

	if p.BreachPriority != nil && higher(tickets.Priorities(), *p.BreachPriority, t.Priority) {
		change.Priority = p.BreachPriority
	}

	if change.Priority == nil && change.Team == nil {
		return nil
	}

	if _, err := w.Apply(ctx, t, change); err != nil {
		return fmt.Errorf("escalate ticket: %w", err)
	}

	return nil
}

/*
escalateConversation raises the conversation's priority and moves it
to the breach team. A team gone since the policy was saved is skipped.
*/
func (a *alarm) escalateConversation(ctx context.Context, c models.Conversation) error {
	if !a.breached {
		return nil
	}

	p := a.policy
	current := string(enums.ConversationPriorityOf(c.Priority))
	if p.BreachPriority != nil && higher(tickets.Priorities(), *p.BreachPriority, current) {
		level := enums.ConversationPriority(*p.BreachPriority)

		updated, _, err := a.writer.SetAttributes(ctx, c, conversations.Attributes{Priority: &level})
		if err != nil {
			return fmt.Errorf("escalate conversation priority: %w", err)
		}

		c = updated
	}

	if p.BreachTeamID == nil || c.AssigneeTeamID != nil && *c.AssigneeTeamID == *p.BreachTeamID {
		return nil
	}

	_, _, err := a.writer.Assign(ctx, c, conversations.Assignment{Member: c.AssigneeMemberID, Team: p.BreachTeamID})
	if err != nil && isRefusal(err) {
		return nil
	}

	return err
}

/*
isRefusal reports whether the conversation refused a change, such as a
team that no longer exists, rather than failing.
*/
func isRefusal(err error) bool {
	return fault.KindOf(err) != fault.Internal
}
