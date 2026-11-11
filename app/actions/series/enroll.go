/*
|--------------------------------------------------------------------------
| Entering A Series
|--------------------------------------------------------------------------
|
| People enter a live series when they begin to match its rules, when
| they do its event, or when a teammate or the API adds them. Whoever
| matches the exit rules does not enter. Each enters once unless the
| series lets people travel it again after a cooldown, and nobody is on
| the same series twice at a time. An account series enrolls the
| company of whoever matched.
|
*/

package series

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
entrant is who enters a series in one go: the series and the version
it runs, the people who matched, how they entered and, for an event,
which one.
*/
type entrant struct {
	row     models.Series
	version models.SeriesVersion
	spec    Spec
	people  []models.Contact
	by      string
	ref     *string
	test    bool
}

/*
admit starts the series for the entrants that may enter and pushes
their first move.
*/
func (s *Series) admit(ctx context.Context, tx *database.Tx, in entrant) ([]models.SeriesEnrollment, error) {
	q := models.New(tx)

	people, err := s.withoutExits(ctx, q, in)
	if err != nil || len(people) == 0 {
		return nil, err
	}

	again := in.spec.Entry.Reentry.Kind == "again"
	since := s.Clock.Now().Add(-time.Duration(max(in.spec.Entry.Reentry.CooldownDays, 0)) * day)

	var rows []models.SeriesEnrollment

	if in.row.Subject == SubjectCompany {
		rows, err = q.EnrollSeriesCompanies(ctx, models.EnrollSeriesCompaniesParams{
			WorkspaceID: in.row.WorkspaceID, SeriesID: in.row.ID, VersionID: in.version.ID, Now: s.Clock.Now(),
			EnteredBy: in.by, TriggerRef: in.ref, Test: in.test, CompanyIds: companiesOf(people), Reentry: again,
			CooldownSince: since,
		})
	} else {
		rows, err = q.EnrollSeriesContacts(ctx, models.EnrollSeriesContactsParams{
			WorkspaceID: in.row.WorkspaceID, SeriesID: in.row.ID, VersionID: in.version.ID, Now: s.Clock.Now(),
			EnteredBy: in.by, TriggerRef: in.ref, Test: in.test, ContactIds: contactIDs(people), Reentry: again,
			CooldownSince: since,
		})
	}

	if err != nil {
		return nil, wrap("enroll", err)
	}

	for _, e := range rows {
		if err := s.Queue.PushTx(ctx, tx, jobs.AdvanceSeriesEnrollmentArgs{
			WorkspaceID: e.WorkspaceID, EnrollmentID: e.ID, Seq: e.Seq,
		}); err != nil {
			return nil, wrap("start journey", err)
		}
	}

	return rows, nil
}

/*
withoutExits leaves out blocked people and those who match the exit
rules, except on a test.
*/
func (s *Series) withoutExits(ctx context.Context, q *models.Queries, in entrant) ([]models.Contact, error) {
	people := make([]models.Contact, 0, len(in.people))
	for _, c := range in.people {
		if c.BlockedAt == nil && (in.row.Subject == SubjectContact || c.CompanyID != nil) {
			people = append(people, c)
		}
	}

	if len(in.spec.Exit.Rules) == 0 || len(people) == 0 || in.test {
		return people, nil
	}

	leaving, err := s.Audiences.Matching(ctx, q, audiences.MatchInput{
		WorkspaceID: in.row.WorkspaceID, Rules: in.spec.Exit.Rules,
	}, people)
	if err != nil {
		return nil, err
	}

	gone := map[ids.UUID]bool{}
	for _, id := range leaving {
		gone[id] = true
	}

	out := people[:0]
	for _, c := range people {
		if !gone[c.ID] {
			out = append(out, c)
		}
	}

	return out, nil
}

/*
entering keeps the people who match a series' entry rules.
*/
func (s *Series) entering(ctx context.Context, q *models.Queries, in entrant) ([]models.Contact, error) {
	spec, people := in.spec, in.people
	if spec.Entry.AudienceID == "" && len(spec.Entry.Rules) == 0 {
		return people, nil
	}

	matched, err := s.Audiences.Matching(ctx, q, audiences.MatchInput{
		WorkspaceID: in.row.WorkspaceID, AudienceID: s.audienceID(spec.Entry.AudienceID), Rules: spec.Entry.Rules,
	}, people)
	if err != nil {
		return nil, err
	}

	keep := map[ids.UUID]bool{}
	for _, id := range matched {
		keep[id] = true
	}

	var out []models.Contact

	for _, c := range people {
		if keep[c.ID] {
			out = append(out, c)
		}
	}

	return out, nil
}

/*
contactIDs lists the contacts' ids.
*/
func contactIDs(people []models.Contact) []ids.UUID {
	out := make([]ids.UUID, len(people))
	for i, c := range people {
		out[i] = c.ID
	}

	return out
}

/*
companiesOf lists the companies of the people, once each.
*/
func companiesOf(people []models.Contact) []ids.UUID {
	seen := map[ids.UUID]bool{}
	var out []ids.UUID

	for _, c := range people {
		if c.CompanyID != nil && !seen[*c.CompanyID] {
			seen[*c.CompanyID] = true
			out = append(out, *c.CompanyID)
		}
	}

	return out
}
