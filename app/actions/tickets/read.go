/*
|--------------------------------------------------------------------------
| Reading Tickets
|--------------------------------------------------------------------------
|
| A page of tickets is filtered by type, kind, state, group, teammate,
| team, company, customer, number, external id, any field value, the
| status of a linked issue, created and resolved ranges and text, newest
| first behind a cursor. One ticket comes with what the panel shows
| beside it: its links, issues, service level clocks and, for trackers,
| who they reach.
|
*/

package tickets

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxPage is the most tickets one page holds.
*/
const maxPage = 100

/*
Filter narrows a page of tickets. Fields matches values exactly, a
field at a time. After is the cursor: the last ticket of the previous
page.
*/
type Filter struct {
	TypeIDs      []ids.UUID
	Kinds        []string
	StateIDs     []ids.UUID
	Groups       []string
	Assignee     *ids.UUID
	Unassigned   bool
	Team         *ids.UUID
	Company      *ids.UUID
	Contact      *ids.UUID
	Number       *int64
	ExternalID   *string
	Priority     *string
	Fields       map[string]any
	IssueGroup   *string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	ResolvedFrom *time.Time
	ResolvedTo   *time.Time
	Query        *string
	Conversation *ids.UUID
	Trashed      bool
	After        *ids.UUID
	Limit        int
}

/*
Page is a page of tickets and the cursor of the next one, nil at the
end.
*/
type Page struct {
	Tickets []Summary
	Next    *ids.UUID
}

/*
Summary is a ticket as a list shows it: its type and state, and for
trackers how many conversations link to it.
*/
type Summary struct {
	Ticket models.Ticket
	Type   models.TicketType
	State  models.TicketState
	Linked int64
	Issues []models.TicketIssue
}

/*
List returns a page of the tickets the member may see.
*/
func (t *Tickets) List(ctx context.Context, m authz.Membership, f Filter) (Page, error) {
	if err := canRead(m); err != nil {
		return Page{}, err
	}

	var out Page

	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		params, err := filterParams(ctx, q, m, f)
		if err != nil {
			return err
		}

		rows, err := q.FilterTickets(ctx, params)
		if err != nil {
			return fmt.Errorf("list tickets: %w", err)
		}

		if len(rows) > int(params.RowLimit)-1 {
			rows = rows[:params.RowLimit-1]
			out.Next = &rows[len(rows)-1].ID
		}

		out.Tickets, err = summarize(ctx, q, rows, t.Clock.Now())

		return err
	})

	return out, err
}

/*
filterParams turns a filter into the query's arguments under the
member's visibility.
*/
func filterParams(
	ctx context.Context, q *models.Queries, m authz.Membership, f Filter,
) (models.FilterTicketsParams, error) {
	access := m.ConversationScope().Params()
	limit := f.Limit
	if limit < 1 || limit > maxPage {
		limit = 25
	}

	p := models.FilterTicketsParams{
		WorkspaceID: m.Workspace.ID, Trashed: f.Trashed, AccessMode: access.Mode, AccessViewer: access.Viewer,
		AccessTeams: access.Teams, AccessUnassigned: access.Unassigned, AccessHiddenInboxes: access.HiddenInboxes,
		TypeIds: orNone(f.TypeIDs), Kinds: nonNil(f.Kinds), StateIds: orNone(f.StateIDs), Groups: nonNil(f.Groups),
		Assignee: f.Assignee, Unassigned: f.Unassigned, Team: f.Team, Company: f.Company, Contact: f.Contact,
		Number: f.Number, ExternalID: f.ExternalID, Priority: f.Priority, Fields: rawObject(f.Fields),
		IssueGroup: f.IssueGroup, CreatedFrom: f.CreatedFrom, CreatedTo: f.CreatedTo, ResolvedFrom: f.ResolvedFrom,
		ResolvedTo: f.ResolvedTo, Query: f.Query, LinkedConversation: f.Conversation, RowLimit: int32(limit + 1),
	}

	if f.After != nil {
		after, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: m.Workspace.ID, ID: *f.After})
		if err != nil {
			return p, exceptions.ErrNotFound.OnField("after")
		}

		p.AfterCreated, p.AfterID = &after.CreatedAt, &after.ID
	}

	return p, nil
}

/*
orNone sends no ids as an empty list.
*/
func orNone(list []ids.UUID) []ids.UUID {
	if list == nil {
		return []ids.UUID{}
	}

	return list
}

/*
summarize adds each ticket's type, state, linked count and issues.
*/
func summarize(
	ctx context.Context, q *models.Queries, rows []models.Ticket, now time.Time,
) ([]Summary, error) {
	if len(rows) == 0 {
		return []Summary{}, nil
	}

	ws := rows[0].WorkspaceID

	setup, err := loadSetup(ctx, q, ws, now)
	if err != nil {
		return nil, err
	}

	ticketIDs := make([]ids.UUID, len(rows))
	for i, r := range rows {
		ticketIDs[i] = r.ID
	}

	counts, err := q.CountLinkedConversations(ctx, models.CountLinkedConversationsParams{
		WorkspaceID: ws, TicketIds: ticketIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("count linked conversations: %w", err)
	}

	issues, err := q.ListIssuesOfTickets(ctx, models.ListIssuesOfTicketsParams{WorkspaceID: ws, TicketIds: ticketIDs})
	if err != nil {
		return nil, fmt.Errorf("list ticket issues: %w", err)
	}

	linked := map[ids.UUID]int64{}
	for _, c := range counts {
		linked[c.FromTicketID] = c.Conversations
	}

	byTicket := map[ids.UUID][]models.TicketIssue{}
	for _, is := range issues {
		byTicket[is.TicketID] = append(byTicket[is.TicketID], is)
	}

	out := make([]Summary, len(rows))
	for i, r := range rows {
		ty, _ := setup.Type(r.TypeID)
		st, _ := setup.State(r.StateID)
		out[i] = Summary{Ticket: r, Type: ty, State: st, Linked: linked[r.ID], Issues: byTicket[r.ID]}
	}

	return out, nil
}

/*
itoa writes a number.
*/
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

/*
errForbidden refuses an action the caller may not take.
*/
func errForbidden() error {
	return exceptions.ErrForbidden
}
