/*
|--------------------------------------------------------------------------
| What Moves Journeys
|--------------------------------------------------------------------------
|
| A change to a contact is checked once against every live series: they
| may begin to match one and enter it, and every journey they or their
| company are on is moved, so exits, goals and waits on rules see the
| change within seconds. An event enters the series it starts, once per
| event. Anything that happens to a series message, sent, opened,
| answered, moves the journey that sent it.
|
*/

package series

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Touched moves a journey whose message changed.
*/
func (s *Series) Touched(ctx context.Context, ws, enrollment ids.UUID) error {
	return s.Queue.Push(context.WithoutCancel(ctx), jobs.AdvanceSeriesEnrollmentArgs{
		WorkspaceID: ws, EnrollmentID: enrollment,
	})
}

/*
CheckSubject checks one contact against every live series.
*/
func (s *Series) CheckSubject(ctx context.Context, a jobs.CheckSeriesSubjectArgs) error {
	if a.ContactID == ids.Nil() && a.CompanyID != nil {
		return s.nudgeCompany(ctx, a.WorkspaceID, *a.CompanyID)
	}

	return s.inTx(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: a.WorkspaceID, ID: a.ContactID})
		if err != nil {
			return gone(err, "find contact")
		}

		live, err := q.ListLiveSeries(ctx, a.WorkspaceID)
		if err != nil {
			return wrap("list live series", err)
		}

		for _, l := range live {
			spec, err := decode(l.SeriesVersion.Spec)
			if err != nil || spec.Entry.Event != "" || !hasEntryRules(spec) {
				continue
			}

			if err := s.offer(ctx, tx, entrant{
				row: l.Series, version: l.SeriesVersion, spec: spec, people: []models.Contact{c}, by: "rules",
			}); err != nil {
				return err
			}
		}

		return s.nudge(ctx, tx, c)
	})
}

/*
OnEvent enters the contact into the live series their event starts.
*/
func (s *Series) OnEvent(ctx context.Context, a jobs.SeriesEventArgs) error {
	return s.inTx(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: a.WorkspaceID, ID: a.ContactID})
		if err != nil {
			return gone(err, "find contact")
		}

		live, err := q.ListLiveSeries(ctx, a.WorkspaceID)
		if err != nil {
			return wrap("list live series", err)
		}

		for _, l := range live {
			spec, err := decode(l.SeriesVersion.Spec)
			if err != nil || spec.Entry.Event != a.Event {
				continue
			}

			ref := a.Event + ":" + a.Ref
			if err := s.offer(ctx, tx, entrant{
				row: l.Series, version: l.SeriesVersion, spec: spec, people: []models.Contact{c}, by: "event",
				ref: &ref,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

/*
offer enters the people of the entrant who match the entry rules.
*/
func (s *Series) offer(ctx context.Context, tx *database.Tx, in entrant) error {
	people, err := s.entering(ctx, models.New(tx), in)
	if err != nil || len(people) == 0 {
		return err
	}

	in.people = people
	_, err = s.admit(ctx, tx, in)

	return err
}

/*
nudge moves every journey of the contact and their company.
*/
func (s *Series) nudge(ctx context.Context, tx *database.Tx, c models.Contact) error {
	rows, err := models.New(tx).ListSubjectEnrollments(ctx, models.ListSubjectEnrollmentsParams{
		WorkspaceID: c.WorkspaceID, ContactID: &c.ID, CompanyID: c.CompanyID,
	})
	if err != nil {
		return wrap("list journeys", err)
	}

	for _, e := range rows {
		if err := s.Queue.PushTx(ctx, tx, jobs.AdvanceSeriesEnrollmentArgs{
			WorkspaceID: e.WorkspaceID, EnrollmentID: e.ID,
		}); err != nil {
			return wrap("nudge journey", err)
		}
	}

	return nil
}

/*
nudgeCompany moves the journeys of a company and of its people.
*/
func (s *Series) nudgeCompany(ctx context.Context, ws, company ids.UUID) error {
	return s.inTx(ctx, ws, func(ctx context.Context, tx *database.Tx) error {
		people, err := models.New(tx).ListCompanySeriesContacts(ctx, models.ListCompanySeriesContactsParams{
			WorkspaceID: ws, CompanyID: &company,
		})
		if err != nil {
			return wrap("list company people", err)
		}

		for _, c := range people {
			if err := s.Queue.PushTx(ctx, tx, jobs.CheckSeriesSubjectArgs{
				WorkspaceID: ws,
				ContactID:   c.ID,
			}); err != nil {
				return wrap("check company person", err)
			}
		}

		return nil
	})
}
