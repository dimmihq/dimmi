/*
|--------------------------------------------------------------------------
| Sweeping
|--------------------------------------------------------------------------
|
| Some rules turn true with time alone, like "trial ends within 5 days",
| so every few minutes each live series with entry rules looks through
| the workspace's contacts for new matches. The same look, run once when
| a series goes live, records who already matches, so only people who
| match later enter. The sweep also pushes any journey whose job was
| lost, and counts goals reached after a journey ended but inside its
| window.
|
*/

package series

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
sweepPage is how many contacts one look reads at a time.
*/
const sweepPage = 500

/*
EnrollAll looks through the workspace's contacts for those who match a
live series' entry rules.
*/
func (s *Series) EnrollAll(ctx context.Context, a jobs.EnrollSeriesArgs) error {
	after := ids.Nil()

	for {
		done := true

		err := s.inTx(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
			in, ok, err := s.liveEntrant(ctx, models.New(tx), a)
			if err != nil || !ok {
				return err
			}

			page, err := s.Audiences.Match(ctx, models.New(tx), audiences.MatchInput{
				WorkspaceID: a.WorkspaceID, AudienceID: s.audienceID(in.spec.Entry.AudienceID),
				Rules: in.spec.Entry.Rules, After: after, Size: sweepPage,
			})
			if err != nil {
				return err
			}

			done, after, in.people = page.Done, page.Last, page.Contacts
			if a.Baseline {
				in.people = matchedBefore(in.people, a.LiveAt)

				return s.baseline(ctx, tx, in)
			}

			_, err = s.admit(ctx, tx, in)

			return err
		})
		if err != nil || done {
			return err
		}
	}
}

/*
liveEntrant reads the series a look is for; ok is false when it is no
longer live.
*/
func (s *Series) liveEntrant(ctx context.Context, q *models.Queries, a jobs.EnrollSeriesArgs) (entrant, bool, error) {
	row, err := find(ctx, q, a.WorkspaceID, a.SeriesID)
	if err != nil || row.Status != StatusLive || row.LiveVersionID == nil {
		return entrant{}, false, gone(err, "find series")
	}

	v, err := q.FindSeriesVersion(ctx, models.FindSeriesVersionParams{
		WorkspaceID: a.WorkspaceID,
		ID:          *row.LiveVersionID,
	})
	if err != nil {
		return entrant{}, false, wrap("find version", err)
	}

	spec, err := decode(v.Spec)
	if err != nil || !hasEntryRules(spec) {
		return entrant{}, false, err
	}

	return entrant{row: row, version: v, spec: spec, by: "rules"}, true, nil
}

/*
matchedBefore keeps the people unchanged since the series went live. A
person added or changed after it may only now match, and the check their
change pushed enters them; the baseline must not shut them out first.
*/
func matchedBefore(people []models.Contact, liveAt *time.Time) []models.Contact {
	if liveAt == nil {
		return people
	}

	return slices.DeleteFunc(people, func(c models.Contact) bool { return c.UpdatedAt.After(*liveAt) })
}

/*
baseline records one page of matches as already matching when the
series went live.
*/
func (s *Series) baseline(ctx context.Context, tx *database.Tx, in entrant) error {
	if len(in.people) == 0 {
		return nil
	}

	p := models.MarkSeriesBaselineParams{
		WorkspaceID: in.row.WorkspaceID, SeriesID: in.row.ID, VersionID: in.version.ID, Subject: in.row.Subject,
		Now: s.Clock.Now(),
	}

	if in.row.Subject == SubjectCompany {
		for _, c := range companiesOf(in.people) {
			p.ContactIds, p.CompanyIds = append(p.ContactIds, ids.Nil()), append(p.CompanyIds, c)
		}
	} else {
		for _, c := range in.people {
			p.ContactIds, p.CompanyIds = append(p.ContactIds, c.ID), append(p.CompanyIds, ids.Nil())
		}
	}

	return wrap("record baseline", models.New(tx).MarkSeriesBaseline(ctx, p))
}

/*
Sweep looks for new matches, pushes lost journeys and counts late
goals in every workspace.
*/
func (s *Series) Sweep(ctx context.Context) error {
	workspaces, err := models.New(s.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces to sweep: %w", err)
	}

	for _, w := range workspaces {
		if err := s.sweepWorkspace(ctx, w.ID); err != nil {
			return err
		}
	}

	return nil
}

/*
sweepWorkspace does the sweep's work in one workspace.
*/
func (s *Series) sweepWorkspace(ctx context.Context, ws ids.UUID) error {
	return s.inTx(ctx, ws, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		live, err := q.ListLiveSeries(ctx, ws)
		if err != nil {
			return wrap("list live series", err)
		}

		for _, l := range live {
			if err := s.sweepOne(ctx, tx, l); err != nil {
				return err
			}
		}

		due, err := q.ListDueSeriesEnrollments(ctx, models.ListDueSeriesEnrollmentsParams{
			WorkspaceID: ws, Before: s.Clock.Now().Add(-2 * time.Minute),
		})
		if err != nil {
			return wrap("list due journeys", err)
		}

		for _, e := range due {
			if err := s.Queue.PushTx(ctx, tx, jobs.AdvanceSeriesEnrollmentArgs{
				WorkspaceID: ws, EnrollmentID: e.ID, Seq: e.Seq,
			}); err != nil {
				return wrap("push due journey", err)
			}
		}

		_, err = s.Broadcasts.ResendDue(ctx, tx, ws)

		return err
	})
}

/*
sweepOne looks for new matches of one live series and counts its late
goals.
*/
func (s *Series) sweepOne(ctx context.Context, tx *database.Tx, l models.ListLiveSeriesRow) error {
	spec, err := decode(l.SeriesVersion.Spec)
	if err != nil {
		return err
	}

	if hasEntryRules(spec) {
		if err := s.Queue.PushTx(ctx, tx, jobs.EnrollSeriesArgs{
			WorkspaceID: l.Series.WorkspaceID, SeriesID: l.Series.ID,
		}); err != nil {
			return wrap("push series look", err)
		}
	}

	if spec.Goal == nil {
		return nil
	}

	return s.lateGoals(ctx, tx, l.Series, spec)
}

/*
lateGoals records the goals reached after a journey ended, inside the
window.
*/
func (s *Series) lateGoals(ctx context.Context, tx *database.Tx, row models.Series, spec Spec) error {
	q := models.New(tx)

	rows, err := q.ListGoalWindowEnrollments(ctx, models.ListGoalWindowEnrollmentsParams{
		WorkspaceID: row.WorkspaceID, SeriesID: row.ID, Since: s.Clock.Now().Add(-time.Duration(spec.Goal.Days) * day),
	})
	if err != nil {
		return wrap("list goal windows", err)
	}

	for _, e := range rows {
		r, err := s.open(ctx, tx, row, e)
		if err != nil {
			continue
		}

		reached, err := r.reachedGoal(ctx)
		if err != nil {
			return err
		}

		if reached {
			if _, err := q.MarkSeriesGoal(ctx, models.MarkSeriesGoalParams{
				WorkspaceID: row.WorkspaceID, ID: e.ID, Now: s.Clock.Now(),
			}); err != nil {
				return wrap("mark late goal", err)
			}
		}
	}

	return nil
}
