/*
|--------------------------------------------------------------------------
| Ticket Facts
|--------------------------------------------------------------------------
|
| Conditions read the conversation's live customer ticket as it is when
| they are evaluated: its type, state and group, priority, team, values,
| whether it waits to close the loop, and the status groups of the
| issues linked to it or to the work linked with it. A conversation
| that is no ticket reads none of them.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/tickets"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ticketFacts is the conversation's ticket with its values and the
status groups of its linked issues.
*/
type ticketFacts struct {
	ticket models.Ticket
	fields map[string]any
	issues []string
}

/*
loadTicket reads the conversation's live customer ticket, nil for none.
*/
func loadTicket(ctx context.Context, q *models.Queries, c models.Conversation) (*ticketFacts, error) {
	t, err := q.FindConversationTicket(ctx, models.FindConversationTicketParams{
		WorkspaceID: c.WorkspaceID, ConversationID: &c.ID,
	})
	if errors.Is(err, database.ErrNotFound) || err == nil && t.DeletedAt != nil {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find conversation ticket: %w", err)
	}

	f := &ticketFacts{ticket: t, fields: map[string]any{}}
	_ = json.Unmarshal(t.Fields, &f.fields)

	f.issues, err = issueGroupsOf(ctx, q, t)

	return f, err
}

/*
issueGroupsOf returns the status groups of the issues linked to the
ticket and to the tickets linked with it, each once.
*/
func issueGroupsOf(ctx context.Context, q *models.Queries, t models.Ticket) ([]string, error) {
	links, err := q.ListTicketLinks(ctx, models.ListTicketLinksParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID, ConversationID: t.ConversationID,
	})
	if err != nil {
		return nil, fmt.Errorf("list ticket links: %w", err)
	}

	related := []ids.UUID{t.ID}
	for _, l := range links {
		related = append(related, l.FromTicketID)
		if l.ToTicketID != nil {
			related = append(related, *l.ToTicketID)
		}
	}

	issues, err := q.ListIssuesOfTickets(ctx, models.ListIssuesOfTicketsParams{
		WorkspaceID: t.WorkspaceID, TicketIds: related,
	})
	if err != nil {
		return nil, fmt.Errorf("list linked issues: %w", err)
	}

	groups := []string{}
	for _, is := range issues {
		if !slices.Contains(groups, is.StatusGroup) {
			groups = append(groups, is.StatusGroup)
		}
	}

	return groups, nil
}

/*
ticketField returns one of the ticket's facts, nothing without a ticket.
*/
func (s *subject) ticketField(name string) any {
	f := s.ticket
	if f == nil {
		return nil
	}

	if key, ok := strings.CutPrefix(name, "fields."); ok {
		return texts(f.fields[key])
	}

	t := f.ticket

	return map[string]any{
		"type_id":  models.PublicID(models.PrefixTicketType, t.TypeID),
		"state_id": models.PublicID(models.PrefixTicketState, t.StateID), "state_group": t.StateGroup,
		"priority": t.Priority, "team_id": publicID(models.PrefixTeam, t.AssigneeTeamID), "close_loop": t.CloseLoop,
		"issue_status_groups": f.issues, "reference": tickets.Reference(t.Number), "title": t.Title,
	}[name]
}

/*
texts turns a list of texts read from JSON into the list conditions
compare; any other value stays as it is.
*/
func texts(v any) any {
	list, ok := v.([]any)
	if !ok {
		return v
	}

	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}

	return out
}
