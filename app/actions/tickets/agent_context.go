/*
|--------------------------------------------------------------------------
| What The AI Agent Knows About Tickets
|--------------------------------------------------------------------------
|
| The AI agent knows the ticket types customers may open and their
| fields, so when the team's guidance says so it collects the required
| values in chat, confirms them with the customer and opens the ticket,
| telling the customer its reference.
|
*/

package tickets

import (
	"context"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AgentField is a field the agent may ask the customer for.
*/
type AgentField struct {
	Key      string
	Label    string
	DataType string
	Options  []string
	Required bool
	Hint     string
}

/*
AgentType is a ticket type customers may open, with the fields the
agent collects.
*/
type AgentType struct {
	ID          ids.UUID
	Name        string
	Description string
	Fields      []AgentField

	rows []models.TicketField
}

/*
AgentTypes returns the live customer types customers may open, with the
fields customers see and may fill. A workspace without a ticket setup
gets the defaults, as every reader of it does.
*/
func AgentTypes(ctx context.Context, q *models.Queries, ws ids.UUID, now time.Time) ([]AgentType, error) {
	setup, err := loadSetup(ctx, q, ws, now)
	if err != nil {
		return nil, err
	}

	var out []AgentType

	for _, ty := range setup.Types {
		if ty.Kind != KindCustomer || !ty.CustomerVisible || ty.ArchivedAt != nil {
			continue
		}

		at := AgentType{ID: ty.ID, Name: ty.Name, Description: ty.Description}

		for _, f := range setup.FieldsOf(ty.ID) {
			if f.ArchivedAt != nil || !f.VisibleCustomer || !f.EditableCustomer || f.DataType == TypeFile {
				continue
			}

			at.rows = append(at.rows, f)
			at.Fields = append(at.Fields, AgentField{
				Key: f.Key, Label: f.Label, DataType: f.DataType, Options: f.Options,
				Required: f.RequiredCreateCustomer, Hint: f.Description,
			})
		}

		out = append(out, at)
	}

	return out, nil
}

/*
Check reads the values the agent collected: the ones that fit, the
required fields still missing and the keys whose value does not fit.
*/
func (ty AgentType) Check(values map[string]any) (map[string]any, []AgentField, []string) {
	clean := map[string]any{}

	for _, f := range ty.rows {
		if v, ok := values[f.Key]; ok && v != nil {
			clean[f.Key] = v
		}
	}

	merged, problems := merge(ty.rows, nil, clean)

	var invalid []string
	for key := range problems {
		invalid = append(invalid, key)
	}

	var missing []AgentField

	for _, f := range ty.Fields {
		if v, ok := merged[f.Key]; f.Required && (!ok || empty(v)) {
			missing = append(missing, f)
		}
	}

	return merged, missing, invalid
}

/*
AgentTicket is a ticket the customer confirmed with the agent.
*/
type AgentTicket struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	TypeID         ids.UUID
	Title          string
	Fields         map[string]any
}

/*
AgentOpen opens the customer's ticket on the conversation, in the
agent's transaction, and returns its reference. A conversation that is
already a ticket returns that ticket's reference.
*/
func (t *Tickets) AgentOpen(ctx context.Context, cw *conversations.Writer, in AgentTicket) (string, error) {
	ws, conversation := in.WorkspaceID, in.ConversationID

	found, err := cw.Q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: ws, ConversationID: &conversation,
	})
	if err == nil && found.DeletedAt == nil {
		return Reference(found.Number), nil
	}

	w, err := t.Wrap(ctx, cw, ws)
	if err != nil {
		return "", err
	}

	made, err := w.Create(ctx, ws, CreateInput{
		TypeID: in.TypeID, Title: strings.TrimSpace(in.Title), Fields: in.Fields, ConversationID: &conversation,
		Customer: true,
	})
	if err != nil {
		return "", err
	}

	return Reference(made.Ticket.Number), nil
}
