/*
|--------------------------------------------------------------------------
| Merging What People Did
|--------------------------------------------------------------------------
|
| When a lead turns out to be a known customer, what the lead did becomes
| part of what the customer did: totals add, first and last times widen,
| daily counts add, and the newest details win. The lead keeps nothing,
| so no rule counts the same event twice.
|
*/

package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/eventroll"
)

/*
Merge is one person turning out to be another.
*/
type Merge struct {
	WorkspaceID ids.UUID
	From        ids.UUID
	Into        ids.UUID
	Now         time.Time
}

/*
MergeSummaries adds the merged person's event summaries to the person
they became, in the caller's transaction.
*/
func MergeSummaries(ctx context.Context, q *models.Queries, m Merge) error {
	rows, err := q.ListContactEventSummaries(ctx, models.ListContactEventSummariesParams{
		WorkspaceID: m.WorkspaceID, ContactIds: []ids.UUID{m.From},
	})
	if err != nil || len(rows) == 0 {
		return wrap("list merged event summaries", err)
	}

	for _, from := range rows {
		if err := mergeOne(ctx, q, m, from); err != nil {
			return err
		}
	}

	return wrap("remove merged event summaries", q.DeleteContactEventSummaries(ctx,
		models.DeleteContactEventSummariesParams{WorkspaceID: m.WorkspaceID, ContactID: m.From}))
}

/*
mergeOne adds one event's summary to the person it now belongs to.
*/
func mergeOne(ctx context.Context, q *models.Queries, m Merge, from models.ListContactEventSummariesRow) error {
	into, err := q.LockEventSummary(ctx, models.LockEventSummaryParams{
		WorkspaceID: m.WorkspaceID, ContactID: m.Into, EventDefinitionID: from.EventDefinitionID,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("lock event summary: %w", err)
	}

	mine := eventroll.Summary{
		Count: into.Count, FirstAt: into.FirstAt, LastAt: into.LastAt, Day: into.DailyDay, Daily: into.Daily,
	}
	theirs := eventroll.Summary{
		Count: from.Count, FirstAt: from.FirstAt, LastAt: from.LastAt, Day: from.DailyDay, Daily: from.Daily,
	}
	s := eventroll.Combine(mine, theirs, m.Now)

	details := into.LastMetadata
	if len(details) == 0 || from.LastAt.After(into.LastAt) {
		details = from.LastMetadata
	}

	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}

	return wrap("save merged event summary", q.SaveEventSummary(ctx, models.SaveEventSummaryParams{
		WorkspaceID: m.WorkspaceID, ContactID: m.Into, EventDefinitionID: from.EventDefinitionID, Count: s.Count,
		FirstAt: s.FirstAt, LastAt: s.LastAt, DailyDay: s.Day, Daily: s.Daily, LastMetadata: details, Now: m.Now,
	}))
}
