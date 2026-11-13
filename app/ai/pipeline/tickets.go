/*
|--------------------------------------------------------------------------
| Tickets For The Agent
|--------------------------------------------------------------------------
|
| When the team's guidance or a procedure asks for tickets, the agent
| gets the open_ticket tool: it names the missing required fields to
| ask for, then a summary for the customer to confirm, and only a
| confirmed call opens the ticket. Like every write, the ticket is
| opened at commit, and the reply's {ticket_reference} becomes its
| reference, such as T-482.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
toolOpenTicket is the ticket tool's name, and ticketReference the
placeholder the reply carries for the new ticket's reference.
*/
const (
	toolOpenTicket  = "open_ticket"
	ticketReference = "{ticket_reference}"
)

/*
loadTicketContext reads, when the team's guidance mentions tickets, the
types the agent may open. A failure costs only this context: the agent
still answers.
*/
func loadTicketContext(ctx context.Context, q *models.Queries, e *env) {
	if !mentionsTickets(e) {
		return
	}

	if types, err := tickets.AgentTypes(ctx, q, e.Conversation.WorkspaceID, e.now); err == nil {
		e.TicketTypes = types
	}
}

/*
mentionsTickets reports whether the agent's guidance or a live
procedure asks for tickets.
*/
func mentionsTickets(e *env) bool {
	if strings.Contains(strings.ToLower(e.Agent.Guidance), "ticket") {
		return true
	}

	for _, p := range e.Procedures {
		raw, _ := json.Marshal(p.def)
		if strings.Contains(strings.ToLower(p.row.Name+" "+string(raw)), "ticket") {
			return true
		}
	}

	return false
}

/*
openTicketSpec describes the ticket tool with the types and fields the
customer may give.
*/
func openTicketSpec(e *env) contracts.ToolSpec {
	names := make([]string, len(e.TicketTypes))

	var b strings.Builder

	for i, ty := range e.TicketTypes {
		names[i] = ty.Name
		fmt.Fprintf(&b, "\n- %s: %s", ty.Name, ty.Description)

		for _, f := range ty.Fields {
			need := "optional"
			if f.Required {
				need = "required"
			}

			fmt.Fprintf(&b, "\n  - %s (%s, %s): %s", f.Key, f.DataType, need, f.Label)

			if len(f.Options) > 0 {
				fmt.Fprintf(&b, "; one of %s", strings.Join(f.Options, ", "))
			}
		}
	}

	schema, _ := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"type", "title", "fields", "confirmed"},
		"properties": map[string]any{
			"type":   enumProperty(names),
			"title":  textProperty("A short title for the ticket in the customer's words, three to eight words."),
			"fields": map[string]any{"type": "object", "description": "Values by field key."},
			"confirmed": map[string]any{"type": "boolean", "description": "true only after the customer " +
				"agreed to the summary you showed them in an earlier message."},
		},
	})

	return contracts.ToolSpec{
		Name: toolOpenTicket,
		Description: "Open a ticket for the customer when the team's guidance says to. Call it with what you know: " +
			"it answers with the required fields still missing (ask the customer for them), then with a summary " +
			"to confirm (show it and ask the customer to confirm), and once called with confirmed true it opens " +
			"the ticket when your reply is sent: write " + ticketReference + " in your reply where the ticket's " +
			"reference goes. Types:" + b.String(),
		Schema: schema,
	}
}

/*
ticketRequest is a ticket the agent asked to open.
*/
type ticketRequest struct {
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Fields    map[string]any `json:"fields"`
	Confirmed bool           `json:"confirmed"`
}

/*
openTicket answers the ticket tool: what is missing, the summary to
confirm, or that the ticket opens with the reply.
*/
func (r *Runner) openTicket(_ context.Context, st *run, args json.RawMessage) (string, error) {
	var in ticketRequest
	if err := json.Unmarshal(args, &in); err != nil || strings.TrimSpace(in.Title) == "" {
		return "", errToolArguments
	}

	i := slices.IndexFunc(st.env.TicketTypes, func(t tickets.AgentType) bool { return t.Name == in.Type })
	if i < 0 {
		return "", errToolArguments
	}

	ty := st.env.TicketTypes[i]
	values, missing, invalid := ty.Check(in.Fields)

	var out map[string]any

	switch {
	case len(invalid) > 0:
		out = map[string]any{
			"status": "invalid_fields", "fields": invalid,
			"next": "ask the customer again for these values in the form the field takes",
		}
	case len(missing) > 0:
		out = map[string]any{
			"status": "missing_fields", "missing": missingFields(missing),
			"next": "ask the customer for these in one message, then call open_ticket again",
		}
	case !in.Confirmed:
		out = map[string]any{"status": "confirm", "summary": map[string]any{
			"type": ty.Name, "title": in.Title, "fields": values,
		}, "next": "show the customer this summary and ask them to confirm; call again with confirmed true"}
	case st.ticket != nil:
		return oneWritePerTurn, nil
	default:
		st.ticket = &tickets.AgentTicket{
			WorkspaceID: st.env.Conversation.WorkspaceID, ConversationID: st.env.Conversation.ID, TypeID: ty.ID,
			Title: in.Title, Fields: values,
		}
		out = map[string]any{"status": "opening", "next": "tell the customer the ticket is open and write " +
			ticketReference + " where its reference goes"}
	}

	raw, err := json.Marshal(out)

	return string(raw), err
}

/*
missingFields names the fields to ask for.
*/
func missingFields(list []tickets.AgentField) []map[string]any {
	out := make([]map[string]any, len(list))
	for i, f := range list {
		out[i] = map[string]any{"key": f.Key, "label": f.Label, "type": f.DataType, "options": f.Options}
	}

	return out
}

/*
applyTicket opens the ticket the agent asked for and writes its
reference into the reply. A ticket the rules refuse leaves the reply
without a reference rather than losing it.
*/
func (r *Runner) applyTicket(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	if st.ticket == nil || r.Tickets == nil {
		d.reply = strings.ReplaceAll(d.reply, ticketReference, "")
		return nil
	}

	ref, err := r.Tickets.AgentOpen(ctx, r.agentWriter(tx, st.env), *st.ticket)

	if _, refused := errors.AsType[*fault.Error](err); refused {
		ref, err = "", nil
	}

	if err != nil {
		return fmt.Errorf("open ticket: %w", err)
	}

	d.reply = strings.ReplaceAll(d.reply, ticketReference, ref)

	return nil
}
