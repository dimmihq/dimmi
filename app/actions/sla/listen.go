/*
|--------------------------------------------------------------------------
| Following The Work
|--------------------------------------------------------------------------
|
| Clocks follow what happens, in the transaction that made it happen. A
| ticket starts its clocks when it is created or given a policy; its
| state pauses and resumes them, resolving meets resolution and ends
| the reply targets, and reopening starts them again. A customer's
| message starts the first or the next reply; a teammate's reply meets
| them and starts the follow-up. A plain conversation does the same
| with its status: snoozed waits, closed resolves.
|
*/

package sla

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Listen hears the ticket and conversation events clocks follow.
*/
func (s *SLA) Listen(d *events.Dispatcher) {
	events.Listen(d, events.InTx, s.onTicketCreated)
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.TicketUpdated) error {
		if _, ok := e.Changes["sla_policy_id"]; !ok {
			return nil
		}

		return s.onTicket(ctx, e.TicketChanged, clocks.begin)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.TicketStateChanged) error {
		return s.onTicket(ctx, e.TicketChanged, clocks.follow)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.TicketResolved) error {
		return s.onTicket(ctx, e.TicketChanged, clocks.resolved)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.TicketReopened) error {
		return s.onTicket(ctx, e.TicketChanged, clocks.begin)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.TicketDeleted) error {
		return s.onTicket(ctx, e.TicketChanged, clocks.cancelAll)
	})
	events.Listen(d, events.InTx, s.onMessage)
	s.listenConversations(d)
}

/*
listenConversations hears the status of plain conversations.
*/
func (s *SLA) listenConversations(d *events.Dispatcher) {
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationClosed) error {
		return s.onPlain(ctx, e.Base, e.ConversationID, clocks.resolved)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationReopened) error {
		return s.onPlain(ctx, e.Base, e.ConversationID, clocks.begin)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationSnoozed) error {
		return s.onPlain(ctx, e.Base, e.ConversationID, clocks.follow)
	})
	events.Listen(d, events.InTx, func(ctx context.Context, e appevents.ConversationUnsnoozed) error {
		return s.onPlain(ctx, e.Base, e.ConversationID, clocks.follow)
	})
}

/*
resolved meets resolution and ends the reply targets: nothing is owed
on work that is done.
*/
func (c clocks) resolved(ctx context.Context) error {
	if err := c.end(ctx, StateMet, TargetResolution); err != nil {
		return err
	}

	return c.end(ctx, StateCanceled, replyTargets()...)
}

/*
cancelAll cancels every open clock.
*/
func (c clocks) cancelAll(ctx context.Context) error {
	return c.end(ctx, StateCanceled, Targets()...)
}

/*
onTicket runs the step on the clocks of a ticket that follows a policy.
*/
func (s *SLA) onTicket(ctx context.Context, e appevents.TicketChanged, step func(clocks, context.Context) error) error {
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		t, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: e.WorkspaceID, ID: e.TicketID})
		if err != nil {
			return nil //nolint:nilerr // a ticket gone has no clocks to follow
		}

		sub, err := ticketSubject(ctx, q, t)
		if err != nil || sub == nil {
			return err
		}

		return step(clocks{q: q, now: s.Clock.Now(), s: sub}, ctx)
	})
}

/*
onPlain runs the step on the clocks of a plain conversation. A
conversation that is a ticket's follows the ticket instead.
*/
func (s *SLA) onPlain(ctx context.Context, b appevents.Base, conv ids.UUID, step func(clocks, context.Context) error,
) error {
	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		sub, err := conversationSubject(ctx, q, b.WorkspaceID, conv)
		if err != nil || sub == nil || sub.ticket != nil {
			return err
		}

		return step(clocks{q: q, now: s.Clock.Now(), s: sub}, ctx)
	})
}

/*
onMessage starts and meets reply targets. A customer's message cancels
the follow-up and starts the first reply, or the next one once the
team has replied; a teammate's public reply meets both and starts the
follow-up.
*/
func (s *SLA) onMessage(ctx context.Context, e appevents.MessageCreated) error {
	if e.Type != string(enums.MessageTypeMessage) {
		return nil
	}

	customer := e.AuthorType == string(enums.AuthorTypeContact) && e.Substantive
	teammate := e.AuthorType == string(enums.AuthorTypeMember)
	if !customer && !teammate {
		return nil
	}

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		sub, err := conversationSubject(ctx, q, e.WorkspaceID, e.ConversationID)
		if err != nil || sub == nil || sub.resolved() {
			return err
		}

		c := clocks{q: q, now: s.Clock.Now(), s: sub}
		if teammate {
			return c.replied(ctx)
		}

		return c.customerWrote(ctx)
	})
}

/*
resolved reports whether the subject's work is done: the ticket
resolved, or the plain conversation closed.
*/
func (s *subject) resolved() bool {
	if s.ticket != nil {
		return s.ticket.StateGroup == "resolved"
	}

	return s.conv.Status == string(enums.ConversationStatusClosed)
}

/*
customerWrote starts the reply the customer now waits for.
*/
func (c clocks) customerWrote(ctx context.Context) error {
	if err := c.end(ctx, StateCanceled, TargetFollowUp); err != nil {
		return err
	}

	target := TargetNextReply
	if c.s.conv != nil && c.s.conv.FirstReplyAt == nil {
		target = TargetFirstReply
	}

	if err := c.start(ctx, target); err != nil {
		return err
	}

	return c.follow(ctx)
}

/*
replied meets the replies owed and starts the follow-up.
*/
func (c clocks) replied(ctx context.Context) error {
	if err := c.end(ctx, StateMet, replyTargets()...); err != nil {
		return err
	}

	if err := c.start(ctx, TargetFollowUp); err != nil {
		return err
	}

	return c.follow(ctx)
}
