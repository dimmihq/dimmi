/*
|--------------------------------------------------------------------------
| People In Audiences
|--------------------------------------------------------------------------
|
| A saved audience's rule can read anything about a person, events and
| company included, so the database cannot narrow a list to its members.
| The list is narrowed by every other filter first, then each person is
| judged as a preview judges them, in the list's order. At most
| AudienceScanCap people are judged, so a list of a very large workspace
| narrowed only by audience covers its first that many people.
|
*/

package contacts

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/audiences"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AudienceScanCap is how many people a list narrowed by audience judges at
most.
*/
const AudienceScanCap = 50_000

/*
judgeBatch is how many people are loaded and judged at once.
*/
const judgeBatch = 500

/*
AudienceFilter keeps people in any of the audiences, or with None, those
in none of them.
*/
type AudienceFilter struct {
	IDs  []ids.UUID `json:"ids"`
	None bool       `json:"none,omitempty"`
}

/*
Judging is one pass over people: the workspace, the moment they are
judged at, the people in order, and the filter.
*/
type Judging struct {
	WorkspaceID ids.UUID
	Now         time.Time
	Rows        []models.Contact
	Filter      AudienceFilter
}

/*
InAudiences keeps the contacts the filter lets through, in their order.
*/
func InAudiences(ctx context.Context, q *models.Queries, j Judging) ([]models.Contact, error) {
	judge, err := audiences.NewJudge(ctx, q, j.WorkspaceID, j.Now)
	if err != nil {
		return nil, err
	}

	var kept []models.Contact
	for start := 0; start < len(j.Rows); start += judgeBatch {
		people, err := audiences.LoadPeople(ctx, q, j.WorkspaceID, j.Rows[start:min(start+judgeBatch, len(j.Rows))])
		if err != nil {
			return nil, err
		}

		for _, p := range people {
			if judge.InAny(p, j.Filter.IDs) != j.Filter.None {
				kept = append(kept, p.Contact)
			}
		}
	}

	return kept, nil
}
