/*
|--------------------------------------------------------------------------
| What The AI Agent Knows About Tickets
|--------------------------------------------------------------------------
|
| The AI agent knows every incident customers can see while it is
| active: its title, the affected areas, the customer label of its state
| and the latest public post, so it answers "Yes, we know, here is the
| latest" without handing off. It also knows the ticket types customers
| may open and their fields, so when the team's guidance says so it
| collects the required values in chat, confirms them with the customer
| and opens the ticket, telling the customer its reference.
|
*/

package tickets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IncidentBrief is an active incident as the agent reads it.
*/
type IncidentBrief struct {
	Title  string
	Areas  []string
	Label  string
	Latest string
	Since  time.Time
}

/*
IncidentBriefs returns the workspace's active incidents customers may
see, newest first.
*/
func IncidentBriefs(ctx context.Context, q *models.Queries, ws ids.UUID) ([]IncidentBrief, error) {
	rows, err := q.ListActiveIncidents(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list active incidents: %w", err)
	}

	var visible []models.Ticket

	for _, r := range rows {
		if r.CustomerVisible {
			visible = append(visible, r)
		}
	}

	if len(visible) == 0 {
		return nil, nil
	}

	labels, latest, err := incidentWords(ctx, q, ws, visible)
	if err != nil {
		return nil, err
	}

	out := make([]IncidentBrief, len(visible))
	for i, r := range visible {
		out[i] = IncidentBrief{
			Title: r.Title, Areas: areasOf(r), Label: labels[r.StateID], Latest: latest[r.ID], Since: r.CreatedAt,
		}
	}

	return out, nil
}

/*
incidentWords reads the customer labels of the states and the latest
public post of each incident.
*/
func incidentWords(
	ctx context.Context, q *models.Queries, ws ids.UUID, list []models.Ticket,
) (map[ids.UUID]string, map[ids.UUID]string, error) {
	states, err := q.ListTicketStates(ctx, ws)
	if err != nil {
		return nil, nil, fmt.Errorf("list ticket states: %w", err)
	}

	labels := map[ids.UUID]string{}
	for _, s := range states {
		labels[s.ID] = s.CustomerLabel
	}

	posts, err := q.ListLatestPublicUpdates(ctx, models.ListLatestPublicUpdatesParams{
		WorkspaceID: ws, TicketIds: ticketIDs(list),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list latest public updates: %w", err)
	}

	latest := map[ids.UUID]string{}
	for _, p := range posts {
		latest[p.TicketID] = p.Body
	}

	return labels, latest, nil
}

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
