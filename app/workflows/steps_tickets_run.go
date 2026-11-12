/*
|--------------------------------------------------------------------------
| Running Ticket Steps
|--------------------------------------------------------------------------
|
| Creating a ticket converts the conversation, titled by the step or by
| the conversation's subject, with values mapped from answers and
| variables. Moving, setting fields, escalating and adding to a tracker
| act on the conversation's own ticket, or do nothing on a conversation
| that is none. Applying an SLA gives the policy to the conversation and
| to its ticket.
|
*/

package workflows

import (
	"context"
	"strconv"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
runCreateTicket makes the conversation a ticket of the step's type.
*/
func runCreateTicket(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	c := settings[workflowspec.CreateTicketSettings](s)

	return st.withTickets(ctx, func(ctx context.Context, w *tickets.Writer) (outcome, error) {
		if _, own, err := st.ownTicket(ctx, w); err != nil || own {
			return skipped(outcomeAlreadyTicket), err
		}

		typeID, ok := st.decode(models.PrefixTicketType, c.TypeID)
		if ty, found := w.Setup.Type(typeID); !ok || !found || ty.Kind != tickets.KindCustomer {
			return skipped(outcomeMissingRef), nil
		}

		in := tickets.CreateInput{TypeID: typeID, ConversationID: &st.conv.ID, ContactID: &st.conv.ContactID}
		if state, ok := st.decode(models.PrefixTicketState, c.StateID); ok {
			in.StateID = &state
		}

		var err error
		if in.Title, err = st.ticketTitle(ctx, c.Title); err != nil {
			return outcome{}, err
		}

		if in.Fields, err = st.values(ctx, w, typeID, c.Fields); err != nil {
			return outcome{}, err
		}

		made, err := w.Create(ctx, st.conv.WorkspaceID, in)
		if err != nil {
			return outcome{}, err
		}

		return created(made.Ticket), nil
	})
}

/*
created is the outcome of a step that made a ticket.
*/
func created(t models.Ticket) outcome {
	return outcome{outcome: outcomeCreated, acted: true, detail: map[string]any{
		"ticket_id": models.PublicID(models.PrefixTicket, t.ID), "reference": tickets.Reference(t.Number),
	}}
}

/*
ticketTitle renders the step's title, or falls back to the
conversation's subject and then its number.
*/
func (st *state) ticketTitle(ctx context.Context, title string) (string, error) {
	text, err := st.render(ctx, title)
	if err != nil || text != "" {
		return text, err
	}

	if st.conv.Subject != nil && *st.conv.Subject != "" {
		return *st.conv.Subject, nil
	}

	return "Conversation #" + strconv.FormatInt(st.conv.Number, 10), nil
}

/*
onOwnTicket runs the change on the conversation's ticket, and does
nothing on a conversation that is none.
*/
func (st *state) onOwnTicket(ctx context.Context, change func(ctx context.Context, w *tickets.Writer,
	t models.Ticket) (outcome, error),
) (outcome, error) {
	return st.withTickets(ctx, func(ctx context.Context, w *tickets.Writer) (outcome, error) {
		t, own, err := st.ownTicket(ctx, w)
		if err != nil || !own {
			return skipped(outcomeNoTicket), err
		}

		return change(ctx, w, t)
	})
}

/*
runSetTicketState moves the ticket to the step's state.
*/
func runSetTicketState(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	state, ok := st.decode(models.PrefixTicketState, settings[workflowspec.SetTicketStateSettings](s).StateID)
	if !ok {
		return skipped(outcomeMissingRef), nil
	}

	return st.onOwnTicket(ctx, func(ctx context.Context, w *tickets.Writer, t models.Ticket) (outcome, error) {
		moved, err := w.Apply(ctx, t, tickets.Change{StateID: &state})

		return changed(moved.StateID != t.StateID), err
	})
}

/*
runSetTicketFields sets the step's field values.
*/
func runSetTicketFields(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	list := settings[workflowspec.SetTicketFieldsSettings](s).Fields

	return st.onOwnTicket(ctx, func(ctx context.Context, w *tickets.Writer, t models.Ticket) (outcome, error) {
		values, err := st.values(ctx, w, t.TypeID, list)
		if err != nil {
			return outcome{}, err
		}

		saved, err := w.Apply(ctx, t, tickets.Change{Fields: values})

		return changed(saved.Version != t.Version), err
	})
}

/*
runEscalate makes back-office work for the ticket.
*/
func runEscalate(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	e := settings[workflowspec.EscalateSettings](s)
	typeID, ok := st.decode(models.PrefixTicketType, e.TypeID)
	if !ok {
		return skipped(outcomeMissingRef), nil
	}

	return st.onOwnTicket(ctx, func(ctx context.Context, w *tickets.Writer, t models.Ticket) (outcome, error) {
		in := tickets.EscalateInput{TypeID: typeID}
		if team, ok := st.decode(models.PrefixTeam, e.TeamID); ok {
			in.TeamID = &team
		}

		var err error
		if in.Title, err = st.ticketTitle(ctx, e.Title); err != nil {
			return outcome{}, err
		}

		if in.Fields, err = st.values(ctx, w, typeID, e.Fields); err != nil {
			return outcome{}, err
		}

		work, err := w.Escalate(ctx, t, in)
		if err != nil {
			return outcome{}, err
		}

		return created(work), nil
	})
}

/*
runAddToTracker links the conversation to the step's tracker.
*/
func runAddToTracker(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	id, ok := st.decode(models.PrefixTicket, settings[workflowspec.AddToTrackerSettings](s).TrackerID)
	if !ok {
		return skipped(outcomeMissingRef), nil
	}

	return st.withTickets(ctx, func(ctx context.Context, w *tickets.Writer) (outcome, error) {
		tracker, err := w.Lock(ctx, st.conv.WorkspaceID, id)
		if err != nil || tracker.DeletedAt != nil || tracker.Kind != tickets.KindTracker {
			return skipped(outcomeMissingRef), nil //nolint:nilerr // a tracker gone is skipped
		}

		_, err = w.Link(ctx, tracker, tickets.LinkInput{Kind: tickets.LinkTracker, ConversationID: &st.conv.ID})

		return outcome{outcome: outcomeDone, acted: true}, err
	})
}

/*
runApplySLA gives the conversation, and its ticket, the step's policy.
*/
func runApplySLA(ctx context.Context, st *state, s workflowspec.Step) (outcome, error) {
	id, ok := st.decode(models.PrefixSLAPolicy, settings[workflowspec.ApplySLASettings](s).SLAPolicyID)
	if !ok || st.e.SLA == nil {
		return skipped(outcomeMissingRef), nil
	}

	var out outcome
	err := st.e.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
		applied, err := st.e.SLA.ApplyToConversation(ctx, st.writer(), st.conv, id)
		out = outcome{outcome: outcomeDone, acted: true, detail: map[string]any{"ticket": applied.Ticket}}

		return err
	})
	if err != nil {
		out, err = refused(err, outcomeMissingRef)
	}

	return out, err
}
