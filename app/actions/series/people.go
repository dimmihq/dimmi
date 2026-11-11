/*
|--------------------------------------------------------------------------
| People On A Series
|--------------------------------------------------------------------------
|
| A teammate or the API can add people to a live series by hand, under
| the same rules as automatic entry, take someone off it, and read
| where everyone stands and what each journey did step by step. Trying
| a series on yourself enrolls the contact with your email as a test:
| it may enter again and again, never counts in the report, and its
| waits can be skipped with a button.
|
*/

package series

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Errors of people on a series.
*/
var (
	ErrNotLive = fault.New(fault.Conflict, "series_not_live", "Set the series live before adding people to it.")
	ErrNoSelf  = fault.New(fault.Invalid, "series_test_contact_missing",
		"No contact has your email yet. Add yourself as a contact, then try again.")
	ErrNotTest = fault.New(fault.Conflict, "series_enrollment_not_test", "Only test journeys can skip a wait.")
)

/*
EnrollInput adds people to a series: contacts by id, or companies by id
for an account series. Test marks a try-out journey.
*/
type EnrollInput struct {
	Membership authz.Membership
	ID         ids.UUID
	ContactIDs []ids.UUID
	CompanyIDs []ids.UUID
	Test       bool
	By         string
}

/*
Enroll adds people to a live series and returns the journeys started.
*/
func (s *Series) Enroll(ctx context.Context, in EnrollInput) ([]models.SeriesEnrollment, error) {
	m := in.Membership
	if !m.Can("series:publish") && (!in.Test || !m.Can("series:write")) {
		return nil, exceptions.ErrForbidden
	}

	var out []models.SeriesEnrollment

	err := s.inTx(ctx, m.Workspace.ID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		in2, err := s.liveSeries(ctx, q, m.Workspace.ID, in.ID)
		if err != nil {
			return err
		}

		in2.people, err = s.peopleOf(ctx, q, m.Workspace.ID, in)
		if err != nil {
			return err
		}

		in2.by, in2.test = in.By, in.Test
		out, err = s.admit(ctx, tx, in2)

		return err
	})

	return out, err
}

/*
TrySelf enrolls the teammate's own contact as a test journey.
*/
func (s *Series) TrySelf(ctx context.Context, m authz.Membership, id ids.UUID) (models.SeriesEnrollment, error) {
	var contact models.Contact

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		me, err := q.FindBroadcastSender(ctx, models.FindBroadcastSenderParams{
			WorkspaceID: m.Workspace.ID,
			ID:          m.Member.ID,
		})
		if err != nil || me.Email == "" {
			return ErrNoSelf
		}

		contact, err = q.FindMemberContact(ctx, models.FindMemberContactParams{
			WorkspaceID: m.Workspace.ID,
			Email:       me.Email,
		})
		if err != nil {
			return ErrNoSelf
		}

		return s.endTests(ctx, q, id, contact)
	})
	if err != nil {
		return models.SeriesEnrollment{}, err
	}

	in := EnrollInput{Membership: m, ID: id, ContactIDs: []ids.UUID{contact.ID}, Test: true, By: "test"}
	if contact.CompanyID != nil {
		in.CompanyIDs = []ids.UUID{*contact.CompanyID}
	}

	rows, err := s.Enroll(ctx, in)
	if err != nil || len(rows) == 0 {
		return models.SeriesEnrollment{}, err
	}

	return rows[0], nil
}

/*
Remove takes someone off a series.
*/
func (s *Series) Remove(ctx context.Context, m authz.Membership, series, id ids.UUID) error {
	return s.onJourney(ctx, journeyRef{m: m, series: series, id: id}, func(ctx context.Context, r *run) error {
		return r.end(ctx, "removed")
	})
}

/*
Skip ends the wait of a test journey now.
*/
func (s *Series) Skip(ctx context.Context, m authz.Membership, series, id ids.UUID) error {
	return s.onJourney(ctx, journeyRef{m: m, series: series, id: id}, func(ctx context.Context, r *run) error {
		if !r.e.Test {
			return ErrNotTest
		}

		if r.e.Status != "waiting" {
			return nil
		}

		w := r.waitOf()
		w.Until = r.now

		return r.hold(ctx, w)
	})
}

/*
journeyRef names one journey of a series for a teammate.
*/
type journeyRef struct {
	m      authz.Membership
	series ids.UUID
	id     ids.UUID
}

/*
onJourney locks one journey of a series and changes it.
*/
func (s *Series) onJourney(ctx context.Context, ref journeyRef, change func(context.Context, *run) error) error {
	m, series, id := ref.m, ref.series, ref.id
	if !m.Can("series:write") {
		return exceptions.ErrForbidden
	}

	return s.inTx(ctx, m.Workspace.ID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		e, err := q.LockSeriesEnrollment(ctx, models.LockSeriesEnrollmentParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil || e.SeriesID != series {
			return exceptions.ErrNotFound
		}

		if !e.Test && !m.Can("series:publish") {
			return exceptions.ErrForbidden
		}

		if ended(e) {
			return nil
		}

		row, err := find(ctx, q, m.Workspace.ID, series)
		if err != nil {
			return err
		}

		r, err := s.open(ctx, tx, row, e)
		if err != nil {
			return err
		}

		return change(ctx, r)
	})
}

/*
liveSeries reads a live series and its version for entering it.
*/
func (s *Series) liveSeries(ctx context.Context, q *models.Queries, ws, id ids.UUID) (entrant, error) {
	row, err := find(ctx, q, ws, id)
	if err != nil {
		return entrant{}, err
	}

	if row.Status != StatusLive || row.LiveVersionID == nil {
		return entrant{}, ErrNotLive
	}

	v, err := q.FindSeriesVersion(ctx, models.FindSeriesVersionParams{WorkspaceID: ws, ID: *row.LiveVersionID})
	if err != nil {
		return entrant{}, wrap("find version", err)
	}

	spec, err := decode(v.Spec)

	return entrant{row: row, version: v, spec: spec}, err
}

/*
peopleOf reads the contacts an enrollment names; for companies, one
contact of each stands for it.
*/
func (s *Series) peopleOf(ctx context.Context, q *models.Queries, ws ids.UUID, in EnrollInput) ([]models.Contact,
	error,
) {
	var out []models.Contact

	for _, id := range in.ContactIDs {
		c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return nil, notFound(err, "find contact")
		}

		out = append(out, c)
	}

	for _, id := range in.CompanyIDs {
		people, err := q.ListCompanySeriesContacts(ctx, models.ListCompanySeriesContactsParams{
			WorkspaceID: ws,
			CompanyID:   &id,
		})
		if err != nil {
			return nil, wrap("list company people", err)
		}

		if len(people) > 0 {
			out = append(out, people[0])
		}
	}

	return out, nil
}

/*
Journey is one enrollment with who travels it.
*/
type Journey = models.ListSeriesEnrollmentPageRow

/*
JourneysInput narrows the journeys of a series. Query searches the
person's name or email and the company's name.
*/
type JourneysInput struct {
	Membership authz.Membership
	ID         ids.UUID
	NodeID     *string
	Status     *string
	Query      *string
	Limit      int
	Offset     int
}

/*
Journeys lists a series' journeys, newest first.
*/
func (s *Series) Journeys(ctx context.Context, in JourneysInput) ([]Journey, error) {
	if !in.Membership.Can("series:read") {
		return nil, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace.ID

	var out []Journey

	err := s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		if _, err := find(ctx, q, ws, in.ID); err != nil {
			return err
		}

		var err error
		out, err = q.ListSeriesEnrollmentPage(ctx, models.ListSeriesEnrollmentPageParams{
			WorkspaceID: ws, SeriesID: in.ID, NodeID: in.NodeID, Status: in.Status, Query: in.Query,
			RowLimit: int64(max(min(in.Limit, 100), 1)), RowOffset: int64(max(in.Offset, 0)),
		})

		return wrap("list journeys", err)
	})

	return out, err
}

/*
Trace returns one journey and its steps in order.
*/
func (s *Series) Trace(ctx context.Context, m authz.Membership, series, id ids.UUID,
) (models.SeriesEnrollment, []models.SeriesStep, error) {
	if !m.Can("series:read") {
		return models.SeriesEnrollment{}, nil, exceptions.ErrForbidden
	}

	var e models.SeriesEnrollment
	var steps []models.SeriesStep

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error

		e, err = q.FindSeriesEnrollment(ctx, models.FindSeriesEnrollmentParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil || e.SeriesID != series {
			return exceptions.ErrNotFound
		}

		steps, err = q.ListEnrollmentSteps(ctx, models.ListEnrollmentStepsParams{
			WorkspaceID: m.Workspace.ID, EnrollmentID: id,
		})

		return wrap("list steps", err)
	})

	return e, steps, err
}

/*
PersonJourneys lists the journeys of a contact and of their company.
*/
func (s *Series) PersonJourneys(ctx context.Context, m authz.Membership, contact, company *ids.UUID,
) ([]models.ListContactSeriesEnrollmentsRow, error) {
	if !m.Can("series:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.ListContactSeriesEnrollmentsRow

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.ListContactSeriesEnrollments(ctx, models.ListContactSeriesEnrollmentsParams{
			WorkspaceID: m.Workspace.ID, ContactID: contact, CompanyID: company,
		})

		return wrap("list person journeys", err)
	})

	return out, err
}

/*
endTests ends the contact's earlier test journeys on the series and
withdraws what they had not sent.
*/
func (s *Series) endTests(ctx context.Context, q *models.Queries, series ids.UUID, c models.Contact) error {
	ended, err := q.EndTestJourneys(ctx, models.EndTestJourneysParams{
		WorkspaceID: c.WorkspaceID, SeriesID: series, ContactID: &c.ID, CompanyID: c.CompanyID, Now: s.Clock.Now(),
	})
	if err != nil {
		return wrap("end test journeys", err)
	}

	for _, e := range ended {
		if _, err := q.WithdrawSeriesRecipients(ctx, models.WithdrawSeriesRecipientsParams{
			WorkspaceID: c.WorkspaceID, SeriesEnrollmentID: &e, Now: s.Clock.Now(),
		}); err != nil {
			return wrap("withdraw test sends", err)
		}
	}

	return nil
}
