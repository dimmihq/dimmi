/*
|--------------------------------------------------------------------------
| Ticket Verdicts
|--------------------------------------------------------------------------
|
| The cheap model helps tickets two ways: it fills a ticket's fields
| from the conversation, each by the hint its field gives, and it says
| which open tracker a new conversation reports. What it is told is in
| resources/prompts/tasks/verdicts/ticket_fill.md and ticket_match.md.
| Customers' words are untrusted data inside <untrusted> and are cut short. Answers outside
| what was asked are dropped by the caller.
|
*/

package verdicts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/support/prompts"
)

/*
Tickets lets the model help with tickets.
*/
type Tickets struct {
	Verdicts *Verdicts
}

/*
Fill returns the values the model read for the fields.
*/
func (t *Tickets) Fill(ctx context.Context, q tickets.FillQuestion) (map[string]any, error) {
	props := map[string]any{}

	var b strings.Builder

	for _, f := range q.Fields {
		props[f.Key] = fieldSchema(f)
		fmt.Fprintf(&b, "- %s (%s): %s. %s\n", f.Key, f.DataType, cut(f.Label, 80), cut(f.Hint, 300))
	}

	schema, err := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"values"},
		"properties": map[string]any{
			"values": map[string]any{"type": "object", "additionalProperties": false, "properties": props},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("write fill schema: %w", err)
	}

	var answer struct {
		Values map[string]any `json:"values"`
	}

	conversation := q.ConversationID
	err = t.Verdicts.ask(ctx, question{
		workspaceID: q.WorkspaceID, kind: "ticket_fill", conversationID: &conversation,
		system: prompts.Task(taskTicketFill, prompts.Vars{"fields": strings.TrimSpace(b.String())}),
		text:   cut(q.Text, maxProfileRunes), schemaName: "ticket_fields",
		schema: schema,
	}, &answer)

	return answer.Values, err
}

/*
fieldSchema is the JSON schema of one field's value.
*/
func fieldSchema(f tickets.FillField) map[string]any {
	switch f.DataType {
	case tickets.TypeNumber, tickets.TypeDecimal, tickets.TypeCurrency:
		return map[string]any{"type": "number"}
	case tickets.TypeBoolean:
		return map[string]any{"type": "boolean"}
	case tickets.TypeSingleChoice:
		return map[string]any{"type": "string", "enum": f.Options}
	case tickets.TypeMultipleChoice:
		return map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": f.Options}}
	}

	return map[string]any{"type": "string"}
}

/*
Match returns the candidates the model says the conversation reports.
*/
func (t *Tickets) Match(ctx context.Context, q tickets.MatchQuestion) ([]ids.UUID, error) {
	choices := make([]string, 0, len(q.Candidates))

	var b strings.Builder

	for _, c := range q.Candidates {
		choices = append(choices, c.ID.String())
		fmt.Fprintf(&b, "- %s: %s. %s\n", c.ID, cut(c.Title, 200), cut(c.Description, 300))
	}

	schema, err := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"ids"},
		"properties": map[string]any{
			"ids": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": choices}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("write match schema: %w", err)
	}

	var answer struct {
		IDs []string `json:"ids"`
	}

	conversation := q.ConversationID
	if err := t.Verdicts.ask(ctx, question{
		workspaceID: q.WorkspaceID, kind: "ticket_match", conversationID: &conversation,
		system: prompts.Task(taskTicketMatch, prompts.Vars{"problems": strings.TrimSpace(b.String())}),
		text:   cut(q.Text, maxProfileRunes), schemaName: "ticket_matches",
		schema: schema,
	}, &answer); err != nil {
		return nil, err
	}

	return parseIDs(answer.IDs), nil
}

/*
parseIDs reads the ids the model gave, dropping anything else.
*/
func parseIDs(list []string) []ids.UUID {
	out := make([]ids.UUID, 0, len(list))
	for _, s := range list {
		if id, err := ids.Parse(s); err == nil {
			out = append(out, id)
		}
	}

	return out
}
