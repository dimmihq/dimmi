/*
|--------------------------------------------------------------------------
| Explaining A Person's Journeys
|--------------------------------------------------------------------------
|
| "Why did Ada get this email?" and "why is Ada not on onboarding?" are
| answered from what was recorded and what holds now: each journey the
| person or their company traveled, the version it followed, every step
| it reached with the path it took, what it waits for, the messages it
| sent and how they were read; and, for each series, how its entry and
| exit rules judge the person right now (explain_entry.go). Nothing is
| written.
|
*/

package series

import (
	"context"
	"errors"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How much one explanation reads.
*/
const (
	explainedJourneys  = 50
	explainedPerSeries = 5
	explainedUntouched = 15
)

/*
ErrExplainWho refuses an explanation that names nobody.
*/
var ErrExplainWho = fault.New(fault.Invalid, "series_explain_who_missing",
	"Name a contact by id or email, or a journey by id.")

/*
ExplainInput names whose journeys to explain: a contact by id or email,
or one journey by id; SeriesID keeps it to one series.
*/
type ExplainInput struct {
	Membership   authz.Membership
	ContactID    *ids.UUID
	Email        string
	EnrollmentID *ids.UUID
	SeriesID     *ids.UUID
}

/*
Explanation is everything about one person's series: the contact, when
one is known, and each series with their journeys on it and how its
rules judge them now.
*/
type Explanation struct {
	Contact *models.Contact
	Series  []Story
}

/*
Story is one series as it concerns the person: the series, the version
that runs now, the person's journeys on it, newest first, and how its
rules judge the contact now.
*/
type Story struct {
	Series   models.Series
	Live     *models.SeriesVersion
	LiveSpec *Spec
	Journeys []JourneyStory
	Entry    *EntryCheck
}

/*
JourneyStory is one journey: the enrollment, the version it follows
with that version's steps, every step it reached in order, and every
message it sent or tried to send.
*/
type JourneyStory struct {
	Enrollment models.ListPersonSeriesJourneysRow
	Version    models.SeriesVersion
	Spec       Spec
	Steps      []models.SeriesStep
	Messages   []models.ListSeriesEnrollmentMessagesRow
}

/*
telling gathers one explanation inside its transaction: the series in
the order they were first met, and the versions already read.
*/
type telling struct {
	q        *models.Queries
	ws       ids.UUID
	stories  map[ids.UUID]*Story
	order    []ids.UUID
	versions map[ids.UUID]models.SeriesVersion
}

/*
Explain reads a person's journeys and how each series judges them now.
*/
func (s *Series) Explain(ctx context.Context, in ExplainInput) (Explanation, error) {
	m := in.Membership
	if !m.Can("series:read") {
		return Explanation{}, exceptions.ErrForbidden
	}

	var out Explanation

	t := &telling{ws: m.Workspace.ID, stories: map[ids.UUID]*Story{}, versions: map[ids.UUID]models.SeriesVersion{}}

	err := s.inWorkspace(ctx, t.ws, func(ctx context.Context, q *models.Queries) error {
		t.q = q

		journeys, contact, err := journeysOf(ctx, q, in)
		if err != nil {
			return err
		}

		out.Contact = contact

		if err := t.journeys(ctx, journeys); err != nil {
			return err
		}

		return t.untouched(ctx, in)
	})
	if err != nil {
		return Explanation{}, err
	}

	for _, id := range t.order {
		story := t.stories[id]
		if out.Contact != nil && story.LiveSpec != nil {
			story.Entry = s.entryCheck(ctx, m, *story, *out.Contact)
		}

		out.Series = append(out.Series, *story)
	}

	return out, nil
}

/*
journeysOf finds the person and their journeys: the journey named, or
every journey of the contact and their company.
*/
func journeysOf(ctx context.Context, q *models.Queries, in ExplainInput,
) ([]models.ListPersonSeriesJourneysRow, *models.Contact, error) {
	ws := in.Membership.Workspace.ID

	if in.EnrollmentID != nil {
		return journeyByID(ctx, q, ws, *in.EnrollmentID)
	}

	contact, err := contactOf(ctx, q, ws, in)
	if err != nil {
		return nil, nil, err
	}

	rows, err := q.ListPersonSeriesJourneys(ctx, models.ListPersonSeriesJourneysParams{
		WorkspaceID: ws, ContactID: &contact.ID, CompanyID: contact.CompanyID, SeriesID: in.SeriesID,
		RowLimit: explainedJourneys,
	})

	return rows, &contact, wrap("list journeys", err)
}

/*
journeyByID finds one journey and the contact who travels it, when a
contact does.
*/
func journeyByID(ctx context.Context, q *models.Queries, ws, id ids.UUID,
) ([]models.ListPersonSeriesJourneysRow, *models.Contact, error) {
	e, err := q.FindSeriesEnrollment(ctx, models.FindSeriesEnrollmentParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil, exceptions.ErrNotFound
	}

	if err != nil {
		return nil, nil, wrap("find journey", err)
	}

	row, err := find(ctx, q, ws, e.SeriesID)
	if err != nil {
		return nil, nil, err
	}

	var contact *models.Contact
	if e.ContactID != nil {
		if c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: *e.ContactID}); err == nil {
			contact = &c
		}
	}

	return []models.ListPersonSeriesJourneysRow{journeyRow(e, row.Name)}, contact, nil
}

/*
contactOf finds the contact named by id or by email.
*/
func contactOf(ctx context.Context, q *models.Queries, ws ids.UUID, in ExplainInput) (models.Contact, error) {
	var (
		c   models.Contact
		err error
	)

	switch email := strings.TrimSpace(in.Email); {
	case in.ContactID != nil:
		c, err = q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: *in.ContactID})
	case email != "":
		c, err = q.FindMemberContact(ctx, models.FindMemberContactParams{WorkspaceID: ws, Email: email})
	default:
		return models.Contact{}, ErrExplainWho
	}

	if errors.Is(err, database.ErrNotFound) || (err == nil && c.DeletedAt != nil) {
		return models.Contact{}, exceptions.ErrNotFound
	}

	return c, wrap("find contact", err)
}

/*
journeys tells each journey under its series, a few per series.
*/
func (t *telling) journeys(ctx context.Context, rows []models.ListPersonSeriesJourneysRow) error {
	for _, j := range rows {
		story, err := t.story(ctx, j.SeriesID)
		if err != nil {
			return err
		}

		if len(story.Journeys) >= explainedPerSeries {
			continue
		}

		told, err := t.journey(ctx, j)
		if err != nil {
			return err
		}

		story.Journeys = append(story.Journeys, told)
	}

	return nil
}

/*
story returns the story of one series, reading the series and the
version it runs the first time it is asked for.
*/
func (t *telling) story(ctx context.Context, id ids.UUID) (*Story, error) {
	if story, ok := t.stories[id]; ok {
		return story, nil
	}

	row, err := find(ctx, t.q, t.ws, id)
	if err != nil {
		return nil, err
	}

	story := &Story{Series: row}
	if row.LiveVersionID != nil {
		v, err := t.version(ctx, *row.LiveVersionID)
		if err != nil {
			return nil, err
		}

		spec, err := decode(v.Spec)
		if err != nil {
			return nil, err
		}

		story.Live, story.LiveSpec = &v, &spec
	}

	t.stories[id] = story
	t.order = append(t.order, id)

	return story, nil
}

/*
version reads one version once.
*/
func (t *telling) version(ctx context.Context, id ids.UUID) (models.SeriesVersion, error) {
	if v, ok := t.versions[id]; ok {
		return v, nil
	}

	v, err := t.q.FindSeriesVersion(ctx, models.FindSeriesVersionParams{WorkspaceID: t.ws, ID: id})
	if err != nil {
		return models.SeriesVersion{}, wrap("find version", err)
	}

	t.versions[id] = v

	return v, nil
}

/*
journey reads what one journey did: its version, its steps and its
messages.
*/
func (t *telling) journey(ctx context.Context, j models.ListPersonSeriesJourneysRow) (JourneyStory, error) {
	v, err := t.version(ctx, j.VersionID)
	if err != nil {
		return JourneyStory{}, err
	}

	spec, err := decode(v.Spec)
	if err != nil {
		return JourneyStory{}, err
	}

	steps, err := t.q.ListEnrollmentSteps(ctx, models.ListEnrollmentStepsParams{
		WorkspaceID: t.ws, EnrollmentID: j.ID,
	})
	if err != nil {
		return JourneyStory{}, wrap("list journey steps", err)
	}

	messages, err := t.q.ListSeriesEnrollmentMessages(ctx, models.ListSeriesEnrollmentMessagesParams{
		WorkspaceID: t.ws, SeriesEnrollmentID: &j.ID,
	})
	if err != nil {
		return JourneyStory{}, wrap("list journey messages", err)
	}

	return JourneyStory{Enrollment: j, Version: v, Spec: spec, Steps: steps, Messages: messages}, nil
}

/*
untouched adds the series the person never traveled, so the answer can
say why they did not enter: the series asked about, or the live series
when none was named.
*/
func (t *telling) untouched(ctx context.Context, in ExplainInput) error {
	switch {
	case in.EnrollmentID != nil:
		return nil
	case in.SeriesID != nil:
		_, err := t.story(ctx, *in.SeriesID)
		return err
	}

	live, err := t.q.ListLiveSeries(ctx, t.ws)
	if err != nil {
		return wrap("list live series", err)
	}

	added := 0
	for _, l := range live {
		if _, ok := t.stories[l.Series.ID]; ok || added >= explainedUntouched {
			continue
		}

		if _, err := t.story(ctx, l.Series.ID); err != nil {
			return err
		}

		added++
	}

	return nil
}

/*
journeyRow is one enrollment in the shape of a person's journey list.
*/
func journeyRow(e models.SeriesEnrollment, name string) models.ListPersonSeriesJourneysRow {
	return models.ListPersonSeriesJourneysRow{
		ID: e.ID, WorkspaceID: e.WorkspaceID, SeriesID: e.SeriesID, VersionID: e.VersionID, Subject: e.Subject,
		ContactID: e.ContactID, CompanyID: e.CompanyID, Status: e.Status, NodeID: e.NodeID, Seq: e.Seq,
		Wait: e.Wait, WakeAt: e.WakeAt, EnteredAt: e.EnteredAt, EnteredBy: e.EnteredBy, TriggerRef: e.TriggerRef,
		GoalAt: e.GoalAt, LeftAt: e.LeftAt, EndReason: e.EndReason, Variants: e.Variants, Test: e.Test,
		StepsExecuted: e.StepsExecuted, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, SeriesName: name,
	}
}
