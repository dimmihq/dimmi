/*
|--------------------------------------------------------------------------
| Events As Rules See Them
|--------------------------------------------------------------------------
|
| Rules read what a person did as event.<name>.count, optionally over
| the last days, and event.<name>.first_at and .last_at. Names may hold
| dots, so the part after the last dot says which. Someone who never did
| it has a count of zero, which "is 0" matches, and no dates.
|
*/

package audiences

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/eventroll"
)

/*
Events is what people did, by contact and then by event name.
*/
type Events map[ids.UUID]map[string]eventroll.Summary

/*
LoadEvents reads the event summaries of the contacts.
*/
func LoadEvents(ctx context.Context, q *models.Queries, ws ids.UUID, contacts []ids.UUID) (Events, error) {
	rows, err := q.ListContactEventSummaries(ctx, models.ListContactEventSummariesParams{
		WorkspaceID: ws, ContactIds: contacts,
	})
	if err != nil {
		return nil, fmt.Errorf("list event summaries: %w", err)
	}

	out := Events{}
	for _, r := range rows {
		if out[r.ContactID] == nil {
			out[r.ContactID] = map[string]eventroll.Summary{}
		}

		out[r.ContactID][r.Name] = eventroll.Summary{
			Count: r.Count, FirstAt: r.FirstAt, LastAt: r.LastAt, Day: r.DailyDay, Daily: r.Daily,
		}
	}

	return out, nil
}

/*
EventValue answers an event field, the part after "event.", for one
person's summaries as of now.
*/
func EventValue(summaries map[string]eventroll.Summary, field string, now time.Time) any {
	field, within, _ := strings.Cut(field, "@")
	cut := strings.LastIndex(field, ".")
	if cut < 0 {
		return nil
	}

	s, done := summaries[field[:cut]]
	switch field[cut+1:] {
	case "count":
		if days, err := strconv.Atoi(within); err == nil && done {
			return float64(eventroll.Within(s, days, now))
		}

		return float64(s.Count)
	case "first_at":
		if done {
			return s.FirstAt
		}
	case "last_at":
		if done {
			return s.LastAt
		}
	}

	return nil
}
