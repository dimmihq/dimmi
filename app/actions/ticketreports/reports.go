/*
|--------------------------------------------------------------------------
| Ticket Reports
|--------------------------------------------------------------------------
|
| How tickets move and what they cost: time spent in each state per
| type, tickets per type and per value of a field, how often service
| levels were kept per policy and target, and which trackers reach the
| most customers, companies and revenue. A member reads them over a
| range of whole days in the workspace's time zone, and counts only the
| tickets they may see, on the same terms as conversations.
|
*/

package ticketreports

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Reports answers ticket reports.
*/
type Reports struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Range is the days a report covers, "2006-01-02", To included; empty
means the last thirty days.
*/
type Range struct {
	From string
	To   string
}

/*
reader reads a report with the queries of the workspace's transaction.
*/
type reader func(ctx context.Context, q *models.Queries, rq reports.Query) error

/*
read checks the member may read tickets, turns the range into a query
and runs the function inside the workspace.
*/
func (r *Reports) read(
	ctx context.Context, m authz.Membership, rg Range, fn reader,
) error {
	if !m.Can("tickets:read") {
		return exceptions.ErrForbidden
	}

	rq, err := reports.NewQuery(m, reports.Filters{From: rg.From, To: rg.To}, r.Clock.Now())
	if err != nil {
		return err
	}

	scoped := tenancy.With(ctx, m.Workspace.ID.String())

	return r.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx), rq)
	})
}

/*
names are the workspace's types and states by id.
*/
type names struct {
	types  map[ids.UUID]models.TicketType
	states map[ids.UUID]models.TicketState
}

/*
loadNames reads the workspace's types and states.
*/
func loadNames(ctx context.Context, q *models.Queries, rq reports.Query) (names, error) {
	ws := rq.Membership.Workspace.ID
	out := names{types: map[ids.UUID]models.TicketType{}, states: map[ids.UUID]models.TicketState{}}

	types, err := q.ListTicketTypes(ctx, ws)
	if err != nil {
		return out, fmt.Errorf("list ticket types: %w", err)
	}

	for _, t := range types {
		out.types[t.ID] = t
	}

	states, err := q.ListTicketStates(ctx, ws)
	if err != nil {
		return out, fmt.Errorf("list ticket states: %w", err)
	}

	for _, s := range states {
		out.states[s.ID] = s
	}

	return out, nil
}

/*
access returns the member's visibility for the report queries.
*/
func access(rq reports.Query) authz.VisibleParams {
	return rq.Membership.ConversationScope().Params()
}
