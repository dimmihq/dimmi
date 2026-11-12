/*
|--------------------------------------------------------------------------
| One Ticket
|--------------------------------------------------------------------------
|
| A ticket read on its own comes with what its panel shows: the tickets
| it links to, its running service level clocks and, for trackers and
| incidents, the customers, companies and revenue they reach.
|
*/

package tickets

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Detail is one ticket with everything its panel shows.
*/
type Detail struct {
	Summary
	Links  []LinkView
	Clocks []models.SlaClock
	Impact *Impact
}

/*
LinkView is a link with the ticket on its other end, when it joins two
tickets.
*/
type LinkView struct {
	Link  models.TicketLink
	Other *models.Ticket
	State *models.TicketState
	Type  *models.TicketType
}

/*
Impact is who a tracker or incident reaches.
*/
type Impact struct {
	Conversations int64
	Customers     int64
	Companies     int64
	Revenue       float64
}

/*
detailOf loads what a ticket's panel shows.
*/
func detailOf(ctx context.Context, q *models.Queries, t models.Ticket, now time.Time) (Detail, error) {
	sums, err := summarize(ctx, q, []models.Ticket{t}, now)
	if err != nil {
		return Detail{}, err
	}

	d := Detail{Summary: sums[0]}

	if d.Links, err = linkViews(ctx, q, t, now); err != nil {
		return d, err
	}

	d.Clocks, err = q.ListOpenSubjectClocks(ctx, models.ListOpenSubjectClocksParams{
		WorkspaceID: t.WorkspaceID, TicketID: &t.ID,
	})
	if err != nil {
		return d, fmt.Errorf("list ticket clocks: %w", err)
	}

	if t.Kind == KindTracker || t.Kind == KindIncident {
		impact, err := impactOf(ctx, q, t)
		if err != nil {
			return d, err
		}

		d.Impact = &impact
	}

	return d, nil
}

/*
impactOf counts who a tracker reaches.
*/
func impactOf(ctx context.Context, q *models.Queries, t models.Ticket) (Impact, error) {
	row, err := q.CountTrackerImpact(ctx, models.CountTrackerImpactParams{WorkspaceID: t.WorkspaceID, TicketID: t.ID})
	if err != nil {
		return Impact{}, fmt.Errorf("count tracker impact: %w", err)
	}

	return Impact{
		Conversations: row.Conversations, Customers: row.Customers, Companies: row.Companies, Revenue: row.Revenue,
	}, nil
}

/*
linkViews loads the ticket's links with the tickets on their other end.
*/
func linkViews(ctx context.Context, q *models.Queries, t models.Ticket, now time.Time) ([]LinkView, error) {
	links, err := q.ListTicketLinks(ctx, models.ListTicketLinksParams{
		WorkspaceID: t.WorkspaceID, TicketID: t.ID, ConversationID: t.ConversationID,
	})
	if err != nil {
		return nil, fmt.Errorf("list ticket links: %w", err)
	}

	setup, err := loadSetup(ctx, q, t.WorkspaceID, now)
	if err != nil {
		return nil, err
	}

	out := make([]LinkView, 0, len(links))
	for _, l := range links {
		v := LinkView{Link: l}

		if other := otherEnd(l, t.ID); other != nil {
			o, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: t.WorkspaceID, ID: *other})
			if err != nil || o.DeletedAt != nil {
				continue
			}

			st, _ := setup.State(o.StateID)
			ty, _ := setup.Type(o.TypeID)
			v.Other, v.State, v.Type = &o, &st, &ty
		}

		out = append(out, v)
	}

	return out, nil
}

/*
otherEnd returns the ticket a link joins to the given one: its target,
or for a link into the ticket or its conversation, where it starts.
*/
func otherEnd(l models.TicketLink, self ids.UUID) *ids.UUID {
	switch {
	case l.FromTicketID == self && l.ToTicketID != nil:
		return l.ToTicketID
	case l.FromTicketID != self:
		id := l.FromTicketID

		return &id
	}

	return nil
}

/*
Load returns any ticket of the workspace as a list shows it, trashed
or not, for webhooks and integrations that act for nobody.
*/
func Load(ctx context.Context, q *models.Queries, ws, id ids.UUID) (Summary, error) {
	t, err := q.FindTicket(ctx, models.FindTicketParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return Summary{}, err
	}

	list, err := summarize(ctx, q, []models.Ticket{t}, t.UpdatedAt)
	if err != nil {
		return Summary{}, err
	}

	return list[0], nil
}
