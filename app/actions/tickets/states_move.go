/*
|--------------------------------------------------------------------------
| Moving Between States
|--------------------------------------------------------------------------
|
| A move checks the type allows the state and, into a resolved state,
| that every field required to resolve has a value, whoever moves it. A
| customer ticket's conversation follows: resolving closes it, leaving
| resolved opens it again. Moving into a state that tells the customer
| schedules the notice two minutes out, so a quick correction sends
| only the last state. A back-office ticket that resolves hands its
| customer ticket back to the team to close the loop.
|
*/

package tickets

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
notifyDelay is how long a customer notice of a new state waits.
*/
const notifyDelay = 2 * time.Minute

/*
moveResult is a ticket after a move and whether it moved.
*/
type moveResult struct {
	ticket models.Ticket
	moved  bool
}

/*
move puts the ticket in the state, enforcing the rules, and reports
whether it moved.
*/
func (w *Writer) move(ctx context.Context, t models.Ticket, to models.TicketState) (moveResult, error) {
	if t.StateID == to.ID {
		return moveResult{ticket: t}, nil
	}

	if !w.Setup.Allows(t.TypeID, to.ID) {
		return moveResult{ticket: t}, ErrStateNotAllowed
	}

	if to.StateGroup == GroupResolved {
		missing := missingFor(w.Setup.FieldsOf(t.TypeID), objectOf(t.Fields), requiredToResolve)
		if len(missing) > 0 {
			return moveResult{ticket: t}, missingError(missing)
		}
	}

	from, _ := w.Setup.State(t.StateID)
	t = w.enter(t, to)

	saved, err := w.save(ctx, t)
	if err != nil {
		return moveResult{ticket: t}, err
	}

	return moveResult{ticket: saved, moved: true}, w.announceMove(ctx, saved, from, to)
}

/*
enter sets the columns of a ticket entering a state.
*/
func (w *Writer) enter(t models.Ticket, to models.TicketState) models.Ticket {
	now := w.Now
	t.StateID, t.StateGroup, t.StateSince = to.ID, to.StateGroup, now
	t.CloseLoop = to.Purpose != nil && *to.Purpose == PurposeCloseLoop
	t.AutoResolveWarnedAt = nil

	if to.StateGroup == GroupResolved {
		t.ResolvedAt = &now
	} else {
		t.ResolvedAt = nil
	}

	return t
}

/*
announceMove logs the move, sends its events, makes the conversation
follow, tells the customer and closes the loop of linked work.
*/
func (w *Writer) announceMove(ctx context.Context, t models.Ticket, from, to models.TicketState) error {
	err := w.log(ctx, t, "state", map[string]any{
		"from": from.ID.String(), "from_name": from.Name, "from_group": from.StateGroup,
		"to": to.ID.String(), "to_name": to.Name, "to_group": to.StateGroup,
	})
	if err != nil {
		return err
	}

	changes := map[string]appevents.TicketChange{
		"state": change(from.ID, to.ID), "group": change(from.StateGroup, to.StateGroup),
	}
	if err := w.emit(ctx, appevents.TicketStateChanged{TicketChanged: w.changed(t, changes)}); err != nil {
		return err
	}

	if err := w.announceGroup(ctx, t, from, to); err != nil {
		return err
	}

	if err := w.follow(ctx, t, to); err != nil {
		return err
	}

	if err := w.scheduleNotice(ctx, t, to); err != nil {
		return err
	}

	return w.closeLoops(ctx, t, to)
}

/*
announceGroup sends resolved or reopened when the move enters or
leaves the resolved group.
*/
func (w *Writer) announceGroup(ctx context.Context, t models.Ticket, from, to models.TicketState) error {
	changes := map[string]appevents.TicketChange{
		"state": change(from.ID, to.ID), "group": change(from.StateGroup, to.StateGroup),
	}

	switch {
	case to.StateGroup == GroupResolved && from.StateGroup != GroupResolved:
		return w.emit(ctx, appevents.TicketResolved{TicketChanged: w.changed(t, changes)})
	case from.StateGroup == GroupResolved && to.StateGroup != GroupResolved:
		return w.emit(ctx, appevents.TicketReopened{TicketChanged: w.changed(t, changes)})
	}

	return nil
}

/*
follow moves a customer ticket's conversation with it: closed when the
ticket resolves, open when it leaves resolved.
*/
func (w *Writer) follow(ctx context.Context, t models.Ticket, to models.TicketState) error {
	if t.ConversationID == nil {
		return nil
	}

	c, err := w.Q.LockConversation(ctx, models.LockConversationParams{
		WorkspaceID: t.WorkspaceID, ID: *t.ConversationID,
	})
	if err != nil || c.DeletedAt != nil {
		return nil //nolint:nilerr // a conversation gone from under the ticket leaves nothing to follow
	}

	status := string(enums.ConversationStatusOpen)
	if to.StateGroup == GroupResolved {
		status = string(enums.ConversationStatusClosed)
	}

	if c.Status == status || (status == string(enums.ConversationStatusOpen) && c.Status != "closed") {
		return nil
	}

	_, _, err = w.ChangeStatus(ctx, c, conversations.StatusChange{Status: status, Reason: conversations.ReasonManual})
	if err != nil {
		return fmt.Errorf("follow ticket state: %w", err)
	}

	return nil
}

/*
scheduleNotice queues the customer notice of a state that tells the
customer, unless the customer moved it themselves.
*/
func (w *Writer) scheduleNotice(ctx context.Context, t models.Ticket, to models.TicketState) error {
	if !to.NotifyCustomer || t.ConversationID == nil || !t.CustomerVisible ||
		w.Actor.Type == string(enums.AuthorTypeContact) || w.Imported {
		return nil
	}

	err := w.Owner.Queue.PushTx(ctx, w.Tx, jobs.NotifyTicketStateArgs{WorkspaceID: t.WorkspaceID, TicketID: t.ID},
		queue.At(w.Now.Add(notifyDelay)))
	if err != nil {
		return fmt.Errorf("schedule ticket notice: %w", err)
	}

	return nil
}

/*
closeLoops hands customer tickets back to the team when a back-office
ticket behind them resolves.
*/
func (w *Writer) closeLoops(ctx context.Context, t models.Ticket, to models.TicketState) error {
	if t.Kind != KindBackOffice || to.StateGroup != GroupResolved {
		return nil
	}

	links, err := w.Q.ListTicketLinks(ctx, models.ListTicketLinksParams{WorkspaceID: t.WorkspaceID, TicketID: t.ID})
	if err != nil {
		return fmt.Errorf("list back-office links: %w", err)
	}

	for _, l := range links {
		if l.Kind != LinkBackOffice || l.ToTicketID == nil || *l.ToTicketID != t.ID {
			continue
		}

		if err := w.returnToTeam(ctx, l.FromTicketID, t); err != nil {
			return err
		}
	}

	return nil
}
