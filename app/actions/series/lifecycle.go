/*
|--------------------------------------------------------------------------
| Pausing, Resuming And Archiving
|--------------------------------------------------------------------------
|
| Pausing stops every message and every step at once and keeps everyone
| where they stand; waits that end meanwhile carry on when the series
| resumes. Archiving ends every journey for good and withdraws what was
| not sent yet. An archived series keeps its versions and its report.
| Any old version can be copied back into the working copy and
| published again.
|
*/

package series

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Pause holds a live series.
*/
func (s *Series) Pause(ctx context.Context, m authz.Membership, id ids.UUID) (models.Series, error) {
	return s.move(ctx, m, id, func(ctx context.Context, tx *database.Tx, row models.Series) (models.Series, error) {
		if row.Status != StatusLive {
			return row, ErrStatus
		}

		now := s.Clock.Now()

		return s.setStatus(ctx, tx, row, models.SetSeriesStatusParams{Status: StatusPaused, PausedAt: &now})
	})
}

/*
Resume sets a paused series live again and moves on everyone whose wait
ended meanwhile.
*/
func (s *Series) Resume(ctx context.Context, m authz.Membership, id ids.UUID) (models.Series, error) {
	return s.move(ctx, m, id, func(ctx context.Context, tx *database.Tx, row models.Series) (models.Series, error) {
		if row.Status != StatusPaused {
			return row, ErrStatus
		}

		out, err := s.setStatus(ctx, tx, row, models.SetSeriesStatusParams{Status: StatusLive})
		if err != nil {
			return out, err
		}

		movable, err := models.New(tx).ListSeriesMovable(ctx, models.ListSeriesMovableParams{
			WorkspaceID: row.WorkspaceID, SeriesID: row.ID, Now: s.Clock.Now(),
		})
		if err != nil {
			return out, wrap("list movable", err)
		}

		for _, e := range movable {
			if err := s.Queue.PushTx(ctx, tx, jobs.AdvanceSeriesEnrollmentArgs{
				WorkspaceID: row.WorkspaceID, EnrollmentID: e.ID,
			}); err != nil {
				return out, wrap("resume traveler", err)
			}
		}

		return out, nil
	})
}

/*
Archive ends a series and every journey on it.
*/
func (s *Series) Archive(ctx context.Context, m authz.Membership, id ids.UUID) (models.Series, error) {
	return s.move(ctx, m, id, func(ctx context.Context, tx *database.Tx, row models.Series) (models.Series, error) {
		if row.Status == StatusArchived {
			return row, nil
		}

		q := models.New(tx)
		now := s.Clock.Now()

		ended, err := q.EndSeriesEnrollments(ctx, models.EndSeriesEnrollmentsParams{
			WorkspaceID: row.WorkspaceID, SeriesID: row.ID, EndReason: new("series_stopped"), Now: now,
		})
		if err != nil {
			return row, wrap("end journeys", err)
		}

		for _, e := range ended {
			if _, err := q.WithdrawSeriesRecipients(ctx, models.WithdrawSeriesRecipientsParams{
				WorkspaceID: row.WorkspaceID, SeriesEnrollmentID: &e, Now: now,
			}); err != nil {
				return row, wrap("withdraw sends", err)
			}
		}

		return s.setStatus(ctx, tx, row, models.SetSeriesStatusParams{
			Status: StatusArchived, ArchivedAt: &now, PausedAt: row.PausedAt,
		})
	})
}

/*
Versions lists a series' published versions, newest first.
*/
func (s *Series) Versions(ctx context.Context, m authz.Membership, id ids.UUID) ([]models.SeriesVersion, error) {
	if !m.Can("series:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.SeriesVersion

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		var err error
		out, err = q.ListSeriesVersions(ctx, models.ListSeriesVersionsParams{WorkspaceID: m.Workspace.ID, SeriesID: id})

		return wrap("list versions", err)
	})

	return out, err
}

/*
Restore copies an old version into the working copy, to publish again.
*/
func (s *Series) Restore(ctx context.Context, m authz.Membership, id, version ids.UUID) (models.Series, error) {
	if !m.Can("series:write") {
		return models.Series{}, exceptions.ErrForbidden
	}

	var spec Spec
	var row models.Series

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error
		if row, err = find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		v, err := q.FindSeriesVersion(ctx, models.FindSeriesVersionParams{WorkspaceID: m.Workspace.ID, ID: version})
		if err != nil || v.SeriesID != id {
			return exceptions.ErrNotFound
		}

		spec, err = decode(v.Spec)

		return err
	})
	if err != nil {
		return row, err
	}

	in := CurrentSettings(row)
	in.Membership, in.Spec = m, &spec

	return s.Save(ctx, in)
}

/*
move locks a series and changes its status as change says, for a
teammate who may publish.
*/
func (s *Series) move(ctx context.Context, m authz.Membership, id ids.UUID,
	change func(context.Context, *database.Tx, models.Series) (models.Series, error),
) (models.Series, error) {
	if !m.Can("series:publish") {
		return models.Series{}, exceptions.ErrForbidden
	}

	var out models.Series

	err := s.inTx(ctx, m.Workspace.ID, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).LockSeries(ctx, models.LockSeriesParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil {
			return notFound(err, "lock series")
		}

		out, err = change(ctx, tx, row)

		return err
	})

	return out, err
}

/*
setStatus stores a series' status and moves its message broadcasts with
it: sending while live, paused while paused, stopped once archived.
*/
func (s *Series) setStatus(ctx context.Context, tx *database.Tx, row models.Series, p models.SetSeriesStatusParams,
) (models.Series, error) {
	q := models.New(tx)
	p.WorkspaceID, p.ID, p.Now = row.WorkspaceID, row.ID, s.Clock.Now()

	if err := q.SetSeriesBroadcastsStatus(ctx, models.SetSeriesBroadcastsStatusParams{
		WorkspaceID: row.WorkspaceID, SeriesID: &row.ID, Status: broadcastStatus(p.Status), Now: p.Now,
	}); err != nil {
		return row, wrap("move series broadcasts", err)
	}

	out, err := q.SetSeriesStatus(ctx, p)

	return out, wrap("set series status", err)
}

/*
broadcastStatus is the status a series' message broadcasts take while
the series has the status.
*/
func broadcastStatus(series string) string {
	switch series {
	case StatusLive:
		return "sending"
	case StatusArchived:
		return "stopped"
	}

	return StatusPaused
}
