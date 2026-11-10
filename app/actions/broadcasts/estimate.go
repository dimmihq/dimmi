/*
|--------------------------------------------------------------------------
| Estimates
|--------------------------------------------------------------------------
|
| Before sending, the review step shows how many the broadcast would
| reach right now and why the rest would be left out, with the same
| checks sending makes and nothing written. A very large workspace is
| counted up to a cap and says so.
|
*/

package broadcasts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
estimateCap is the most contacts an estimate reads.
*/
const estimateCap = 50_000

/*
Estimate is how many a broadcast would reach now, why the others would
be left out, and whether counting stopped at the cap.
*/
type Estimate struct {
	Reach   int
	Skipped map[string]int
	Capped  bool
}

/*
Estimate counts who the broadcast would reach if it were sent now.
*/
func (b *Broadcasts) Estimate(ctx context.Context, m authz.Membership, id ids.UUID) (Estimate, error) {
	if !m.Can("broadcasts:read") {
		return Estimate{}, exceptions.ErrForbidden
	}

	out := Estimate{Skipped: map[string]int{}}

	err := b.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		row, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if row.Channel == ChannelSlack {
			return b.estimateRooms(ctx, q, row, &out)
		}

		return b.estimatePeople(ctx, q, row, &out)
	})

	return out, err
}

/*
estimatePeople counts the contacts a person-to-person broadcast would
reach.
*/
func (b *Broadcasts) estimatePeople(ctx context.Context, q *models.Queries, row models.Broadcast, out *Estimate) error {
	seen := map[string]bool{}
	after, read := ids.Nil(), 0

	for {
		page, err := b.Audiences.Match(ctx, q, audiences.MatchInput{
			WorkspaceID: row.WorkspaceID, AudienceID: row.AudienceID, Rules: row.Rules, After: after, Size: freezePage,
		})
		if err != nil {
			return err
		}

		people, err := b.reach(ctx, q, row, matched{contacts: page.Contacts, seen: seen})
		if err != nil {
			return err
		}

		out.add(people)

		if read += freezePage; page.Done || read >= estimateCap {
			out.Capped = !page.Done

			return nil
		}

		after = page.Last
	}
}

/*
estimateRooms counts the Slack channels a broadcast would post to.
*/
func (b *Broadcasts) estimateRooms(ctx context.Context, q *models.Queries, row models.Broadcast, out *Estimate) error {
	companies, filtered, err := b.audienceCompanies(ctx, row)
	if err != nil {
		return err
	}

	rooms, err := q.ListBroadcastRooms(ctx, models.ListBroadcastRoomsParams{
		WorkspaceID: row.WorkspaceID, RoomIds: nonNil(row.RoomIds),
	})
	if err != nil {
		return wrap("list broadcast rooms", err)
	}

	for _, room := range rooms {
		out.add([]candidate{{reason: roomReason(room, companies, filtered)}})
	}

	return nil
}

/*
add counts candidates as reached or left out by reason.
*/
func (e *Estimate) add(people []candidate) {
	for _, c := range people {
		if c.reason == "" {
			e.Reach++

			continue
		}

		e.Skipped[c.reason]++
	}
}
