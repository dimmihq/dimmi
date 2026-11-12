/*
|--------------------------------------------------------------------------
| Applying Policies
|--------------------------------------------------------------------------
|
| A new ticket follows the policy it was created with, or else the first
| policy, in the workspace's order, that names its type or its priority.
| A policy that names neither applies only when set by hand or by a
| workflow. A workflow's apply SLA step gives the policy to the
| conversation and to its ticket when it has one; the ticket's clocks
| then run the conversation's replies.
|
*/

package sla

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/tickets"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
onTicketCreated starts the clocks of a ticket made with a policy, or
gives it the first policy that matches.
*/
func (s *SLA) onTicketCreated(ctx context.Context, e appevents.TicketCreated) error {
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		t, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: e.WorkspaceID, ID: e.TicketID})
		if err != nil {
			return nil //nolint:nilerr // a ticket gone has no clocks to start
		}

		if t.SlaPolicyID != nil {
			sub, err := ticketSubject(ctx, q, t)
			if err != nil || sub == nil {
				return err
			}

			return clocks{q: q, now: s.Clock.Now(), s: sub}.begin(ctx)
		}

		policy, ok, err := matching(ctx, q, t)
		if err != nil || !ok {
			return err
		}

		return s.setTicketPolicy(ctx, tx, e.Base, policyOf{ticket: t, policy: policy})
	})
}

/*
matching returns the first live policy naming the ticket's type or
priority, and false for none.
*/
func matching(ctx context.Context, q *models.Queries, t models.Ticket) (ids.UUID, bool, error) {
	list, err := q.ListSLAPolicies(ctx, t.WorkspaceID)
	if err != nil {
		return ids.UUID{}, false, fmt.Errorf("list sla policies: %w", err)
	}

	for _, p := range list {
		if p.ArchivedAt != nil || len(p.TicketTypeIds) == 0 && len(p.Priorities) == 0 {
			continue
		}

		if (len(p.TicketTypeIds) == 0 || slices.Contains(p.TicketTypeIds, t.TypeID)) &&
			(len(p.Priorities) == 0 || slices.Contains(p.Priorities, t.Priority)) {
			return p.ID, true, nil
		}
	}

	return ids.UUID{}, false, nil
}

/*
policyOf is a ticket and the policy it is to follow.
*/
type policyOf struct {
	ticket models.Ticket
	policy ids.UUID
}

/*
setTicketPolicy gives the ticket the policy through the tickets writer,
which logs it and announces the change its clocks start from.
*/
func (s *SLA) setTicketPolicy(ctx context.Context, tx *database.Tx, b appevents.Base, p policyOf) error {
	t := p.ticket
	w, err := s.Tickets.AutomationWriter(ctx, tx, t.WorkspaceID)
	if err != nil {
		return err
	}

	w.Chain = b.WorkflowChain

	locked, err := w.Lock(ctx, t.WorkspaceID, t.ID)
	if err != nil {
		return err
	}

	_, err = w.Apply(ctx, locked, tickets.Change{SLAPolicyID: &p.policy})

	return err
}

/*
Applied says what an apply SLA step changed: the conversation, and the
ticket when the conversation is one.
*/
type Applied struct {
	Ticket bool
}

/*
ApplyToConversation gives the conversation the policy within the
writer's transaction, and its live customer ticket too, so the clocks
run on the ticket. Changes carry the writer's actor and chain.
*/
func (s *SLA) ApplyToConversation(ctx context.Context, w *conversations.Writer, c models.Conversation, policy ids.UUID,
) (Applied, error) {
	p, err := findPolicy(ctx, w.Q, c.WorkspaceID, policy)
	if err != nil {
		return Applied{}, err
	}

	if p.ArchivedAt != nil {
		return Applied{}, ErrPolicyArchived
	}

	err = w.Q.SetConversationSLAPolicy(ctx, models.SetConversationSLAPolicyParams{
		ConversationID: c.ID, WorkspaceID: c.WorkspaceID, PolicyID: policy, Now: w.Now,
	})
	if err != nil {
		return Applied{}, fmt.Errorf("set conversation sla policy: %w", err)
	}

	t, err := w.Q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: c.WorkspaceID, ConversationID: &c.ID,
	})
	if err == nil && t.DeletedAt == nil {
		return Applied{Ticket: true}, s.applyToTicket(ctx, w, t, policy)
	}

	sub, err := conversationSubject(ctx, w.Q, c.WorkspaceID, c.ID)
	if err != nil || sub == nil {
		return Applied{}, err
	}

	return Applied{}, clocks{q: w.Q, now: w.Now, s: sub}.begin(ctx)
}

/*
applyToTicket sets the policy on the conversation's ticket through a
ticket writer wrapping the caller's.
*/
func (s *SLA) applyToTicket(ctx context.Context, w *conversations.Writer, t models.Ticket, policy ids.UUID) error {
	tw, err := s.Tickets.Wrap(ctx, w, t.WorkspaceID)
	if err != nil {
		return err
	}

	locked, err := tw.Lock(ctx, t.WorkspaceID, t.ID)
	if err != nil {
		return err
	}

	_, err = tw.Apply(ctx, locked, tickets.Change{SLAPolicyID: &policy})

	return err
}
