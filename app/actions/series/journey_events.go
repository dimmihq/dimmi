/*
|--------------------------------------------------------------------------
| Series Events
|--------------------------------------------------------------------------
|
| Publishing a version and a journey ending each dispatch an event in
| the transaction of the change, so webhooks hear about the series and
| the journey. Test journeys dispatch nothing.
|
*/

package series

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
announce dispatches the event in the transaction, when events are
wired.
*/
func (s *Series) announce(ctx context.Context, tx *database.Tx, ev appevents.Named) error {
	if s.Events == nil {
		return nil
	}

	return wrap("dispatch "+ev.EventName(), s.Events.Dispatch(ctx, tx, ev))
}

/*
published announces the live version the member just published.
*/
func (s *Series) published(ctx context.Context, tx *database.Tx, row models.Series, member ids.UUID) error {
	if row.LiveVersionID == nil {
		return nil
	}

	return s.announce(ctx, tx, appevents.SeriesPublished{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: row.WorkspaceID, OccurredAt: s.Clock.Now(),
			Actor: appevents.Actor{Type: "member", ID: member},
		},
		SeriesID: row.ID, VersionID: *row.LiveVersionID,
	})
}

/*
ended announces a journey that just ended, unless it was a test.
*/
func (r *run) ended(ctx context.Context, reason string) error {
	if r.e.Test {
		return nil
	}

	return r.s.announce(ctx, r.tx, appevents.SeriesJourneyCompleted{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: r.e.WorkspaceID, OccurredAt: r.now,
			Actor: appevents.Actor{Type: "system", ID: ids.Nil()},
		},
		SeriesID: r.e.SeriesID, EnrollmentID: r.e.ID, Reason: reason, Goal: r.e.GoalAt != nil,
	})
}
