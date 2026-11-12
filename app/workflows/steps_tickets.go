/*
|--------------------------------------------------------------------------
| Ticket Steps
|--------------------------------------------------------------------------
|
| Ticket steps work the conversation's customer ticket through the
| tickets writer, wrapping the run's own writer: the change carries the
| workflow's name and chain, so it never starts the same workflow again.
| Each runs in a savepoint. A change the ticket refuses, a required
| field left empty or a state its type does not use, is rolled back and
| recorded as a failed step with the reason, and the run goes on.
|
*/

package workflows

import (
	"context"
	"errors"
	"maps"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The outcomes of ticket steps.
*/
const (
	outcomeNoTicket      = "no_ticket"
	outcomeAlreadyTicket = "already_ticket"
	outcomeCreated       = "created"
)

/*
ticketExecutors returns the executors of the ticket and SLA steps.
*/
func ticketExecutors() map[string]executor {
	return map[string]executor{
		workflowspec.KindCreateTicket: runCreateTicket, workflowspec.KindSetTicketState: runSetTicketState,
		workflowspec.KindSetTicketFields: runSetTicketFields, workflowspec.KindEscalate: runEscalate,
		workflowspec.KindAddToTracker: runAddToTracker, workflowspec.KindApplySLA: runApplySLA,
	}
}

/*
ticketStep is a step's work on the tickets writer.
*/
type ticketStep func(ctx context.Context, w *tickets.Writer) (outcome, error)

/*
withTickets runs the work in a savepoint with a tickets writer acting
for the workflow. A refusal rolls the work back and fails the step; the
conditions read the ticket afresh afterwards.
*/
func (st *state) withTickets(ctx context.Context, work ticketStep) (outcome, error) {
	if st.e.Tickets == nil {
		return skipped(outcomeMissingRef), nil
	}

	var out outcome

	err := st.e.DB.Transaction(ctx, func(ctx context.Context, _ *database.Tx) error {
		w, err := st.e.Tickets.Wrap(ctx, st.writer(), st.conv.WorkspaceID)
		if err != nil {
			return err
		}

		out, err = work(ctx, w)

		return err
	})

	st.subj = nil
	if err != nil && fault.KindOf(err) != fault.Internal {
		return failed(err), nil
	}

	if err != nil {
		return outcome{}, err
	}

	if conv, err := st.q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: st.conv.WorkspaceID, ID: st.conv.ID,
	}); err == nil {
		st.conv = conv
	}

	return out, nil
}

/*
failed is the outcome of a change the ticket refused, with its reason.
*/
func failed(err error) outcome {
	detail := map[string]any{"code": fault.CodeOf(err), "message": err.Error()}
	if fe, ok := errors.AsType[*fault.Error](err); ok {
		detail["message"] = fe.Message()
		maps.Copy(detail, fe.Meta())
	}

	return outcome{status: "failed", outcome: fault.CodeOf(err), detail: detail, acted: true}
}

/*
ownTicket locks the conversation's live customer ticket, false for none.
*/
func (st *state) ownTicket(ctx context.Context, w *tickets.Writer) (models.Ticket, bool, error) {
	found, err := w.Q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: st.conv.WorkspaceID, ConversationID: &st.conv.ID,
	})
	if err != nil || found.DeletedAt != nil {
		return models.Ticket{}, false, nil //nolint:nilerr // a conversation that is no ticket has none to work
	}

	t, err := w.Lock(ctx, found.WorkspaceID, found.ID)

	return t, err == nil, err
}

/*
values renders the step's field values for the ticket type, reading
each as its field holds it: numbers, yes or no, and lists of choices.
*/
func (st *state) values(ctx context.Context, w *tickets.Writer, typeID ids.UUID, list []workflowspec.TicketValue,
) (map[string]any, error) {
	if len(list) == 0 {
		return nil, nil
	}

	types := map[string]string{}
	for _, f := range w.Setup.FieldsOf(typeID) {
		types[f.Key] = f.DataType
	}

	out := map[string]any{}
	for _, v := range list {
		text, err := st.render(ctx, v.Value)
		if err != nil {
			return nil, err
		}

		out[v.Key] = typed(types[v.Key], strings.TrimSpace(text))
	}

	return out, nil
}

/*
typed reads rendered text as a field of the type holds it; text that
does not parse stays text, for the ticket to refuse.
*/
func typed(dataType, text string) any {
	switch dataType {
	case tickets.TypeNumber, tickets.TypeDecimal, tickets.TypeCurrency:
		if n, err := strconv.ParseFloat(text, 64); err == nil {
			return n
		}
	case tickets.TypeBoolean:
		switch strings.ToLower(text) {
		case "true", "yes":
			return true
		case "false", "no":
			return false
		}
	case tickets.TypeMultipleChoice:
		out := []any{}
		for part := range strings.SplitSeq(text, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}

		return out
	}

	return text
}
