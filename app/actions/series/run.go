/*
|--------------------------------------------------------------------------
| One Journey At A Time
|--------------------------------------------------------------------------
|
| Moving a journey reads everything it needs once, inside the lock on the
| enrollment: the series, the version the traveler follows, and the
| traveler, a contact or a company with its people. Rules are always
| asked against the traveler as they are now, never as they were when
| they entered.
|
*/

package series

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
run is one journey being moved: the series, the enrollment, the graph
of its version, the traveler, the time it is moved at, the steps
taken in this job, and whether the job is the timer of a wait.
*/
type run struct {
	s        *Series
	tx       *database.Tx
	q        *models.Queries
	row      models.Series
	e        models.SeriesEnrollment
	g        graph
	people   []models.Contact
	company  *ids.UUID
	variants map[string]string
	now      time.Time
	steps    int
	timer    bool
}

/*
open reads what moving one enrollment needs.
*/
func (s *Series) open(ctx context.Context, tx *database.Tx, row models.Series, e models.SeriesEnrollment) (*run,
	error,
) {
	q := models.New(tx)

	g, err := s.version(ctx, q, e.WorkspaceID, e.VersionID)
	if err != nil {
		return nil, err
	}

	r := &run{s: s, tx: tx, q: q, row: row, e: e, g: g, now: s.Clock.Now(), variants: map[string]string{}}
	_ = json.Unmarshal(e.Variants, &r.variants)

	if err := r.loadPeople(ctx); err != nil {
		return nil, err
	}

	return r, nil
}

/*
loadPeople reads the traveler: the contact and their company, or the
company and its people.
*/
func (r *run) loadPeople(ctx context.Context) error {
	ws := r.e.WorkspaceID
	if r.e.Subject == SubjectContact && r.e.ContactID != nil {
		c, err := r.q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: *r.e.ContactID})
		if err != nil {
			return notFound(err, "find traveler")
		}

		r.people, r.company = []models.Contact{c}, c.CompanyID

		return nil
	}

	if r.e.CompanyID == nil {
		return nil
	}

	r.company = r.e.CompanyID

	people, err := r.q.ListCompanySeriesContacts(ctx, models.ListCompanySeriesContactsParams{
		WorkspaceID: ws, CompanyID: r.e.CompanyID,
	})
	r.people = people

	return wrap("list company people", err)
}

/*
matches reports whether the traveler, or for an account anyone in it,
matches the rules.
*/
func (r *run) matches(ctx context.Context, audience *ids.UUID, rules json.RawMessage) (bool, error) {
	got, err := r.s.Audiences.Matching(ctx, r.q, audiences.MatchInput{
		WorkspaceID: r.e.WorkspaceID, AudienceID: audience, Rules: rules,
	}, r.people)

	return len(got) > 0, err
}

/*
didEvent reports whether the traveler, or anyone in the account, did
the event since the time.
*/
func (r *run) didEvent(ctx context.Context, name string, since time.Time) (bool, error) {
	if name == "" || len(r.people) == 0 {
		return false, nil
	}

	people := make([]ids.UUID, len(r.people))
	for i, c := range r.people {
		people[i] = c.ID
	}

	did, err := r.q.ContactEventSince(ctx, models.ContactEventSinceParams{
		WorkspaceID: r.e.WorkspaceID, ContactIds: people, Name: name, Since: since,
	})

	return did, wrap("check event", err)
}

/*
version reads a version's graph.
*/
func (s *Series) version(ctx context.Context, q *models.Queries, ws, id ids.UUID) (graph, error) {
	v, err := q.FindSeriesVersion(ctx, models.FindSeriesVersionParams{WorkspaceID: ws, ID: id})
	if err != nil {
		return graph{}, notFound(err, "find series version")
	}

	spec, err := decode(v.Spec)
	if err != nil {
		return graph{}, err
	}

	return compile(spec), nil
}

/*
audienceID reads the saved audience an entry names, nil when none.
*/
func (s *Series) audienceID(public string) *ids.UUID {
	if public == "" {
		return nil
	}

	id, err := s.IDs.Decode(string(models.PrefixAudience), public)
	if err != nil {
		return nil
	}

	return &id
}

/*
moveParams starts a move of an enrollment from where it stands.
*/
func moveParams(e models.SeriesEnrollment, now time.Time) models.MoveSeriesEnrollmentParams {
	return models.MoveSeriesEnrollmentParams{
		WorkspaceID: e.WorkspaceID, ID: e.ID, VersionID: e.VersionID, NodeID: e.NodeID, Status: e.Status,
		Wait: e.Wait, WakeAt: e.WakeAt, Variants: e.Variants, GoalAt: e.GoalAt, LeftAt: e.LeftAt,
		EndReason: e.EndReason, Now: now,
	}
}

/*
notFound turns a missing row into not found.
*/
func notFound(err error, what string) error {
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrNotFound
	}

	return wrap(what, err)
}

/*
ended reports whether a journey is over.
*/
func ended(e models.SeriesEnrollment) bool {
	return e.Status == "finished" || e.Status == "exited"
}
