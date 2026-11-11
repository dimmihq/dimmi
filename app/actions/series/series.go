/*
|--------------------------------------------------------------------------
| Series
|--------------------------------------------------------------------------
|
| A series is built as a working copy that saves freely and goes live by
| publishing a version. Reading takes series:read, building series:write
| and anything that reaches customers, publishing, pausing and enrolling
| people, series:publish, so a teammate can build a journey that an
| admin sets live.
|
*/

package series

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/actions/broadcasts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
)

/*
Who travels a series.
*/
const (
	SubjectContact = "contact"
	SubjectCompany = "company"
)

/*
Statuses a series moves through.
*/
const (
	StatusDraft    = "draft"
	StatusLive     = "live"
	StatusPaused   = "paused"
	StatusArchived = "archived"
)

/*
Errors the series API returns.
*/
var (
	ErrName    = fault.New(fault.Invalid, "series_name_invalid", "Give the series a name of up to 120 characters.")
	ErrSubject = fault.New(fault.Invalid, "series_subject_invalid",
		"A series is for contacts or for companies.")
	ErrNameTaken = fault.New(fault.Conflict, "series_name_taken", "Another series already has this name.")
	ErrConflict  = fault.New(fault.Conflict, "edit_conflict",
		"Someone else saved this series meanwhile. Reload it to see their changes.")
	ErrArchived = fault.New(fault.Conflict, "series_archived", "This series is archived and can no longer change.")
	ErrNotDraft = fault.New(fault.Conflict, "series_not_draft",
		"This series went live once, so it is archived rather than deleted.")
	ErrProblems = fault.New(fault.Invalid, "series_invalid", "Fix what the series misses before it goes live.")
	ErrStatus   = fault.New(fault.Conflict, "series_status_invalid", "The series cannot do that in its status.")
)

/*
PathPicker lets an AI pick one path for a person.
*/
type PathPicker interface {
	/*
		Pick returns the id of the path the AI chose for the profile, or
		an error when it could not choose.
	*/
	Pick(ctx context.Context, in PickInput) (string, error)
}

/*
PickInput is what the AI reads: the workspace that pays for it, the
team's instructions, the paths to choose from and the person's profile.
*/
type PickInput struct {
	WorkspaceID ids.UUID
	Prompt      string
	Paths       []AIPath
	Profile     string
}

/*
Series holds every series action. DB stores them, Clock dates them,
Queue runs their jobs, Events tells the rest of the app, Audiences
decides who matches, Broadcasts sends their messages, IDs reads the
public ids a definition names, Limiter counts messages per person,
Picker is the AI that picks paths, HTTP posts webhooks, and Config sets
the limits.
*/
type Series struct {
	DB         *database.DB
	Clock      clock.Clock
	Queue      *queue.Queue
	Events     *events.Dispatcher
	Audiences  *audiences.Audiences
	Broadcasts *broadcasts.Broadcasts
	IDs        *ids.Registry
	Limiter    *ratelimit.Limiter
	Picker     PathPicker
	HTTP       *httpclient.Client
	Config     config.SeriesConfig
}

/*
ListInput narrows the list of series.
*/
type ListInput struct {
	Membership authz.Membership
	Status     *string
	Search     *string
	Limit      int
	Offset     int
}

/*
List returns a page of the workspace's series, most recently changed
first.
*/
func (s *Series) List(ctx context.Context, in ListInput) ([]models.Series, error) {
	if !in.Membership.Can("series:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.Series

	err := s.inWorkspace(ctx, in.Membership.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error

		out, err = q.ListSeriesPage(ctx, models.ListSeriesPageParams{
			WorkspaceID: in.Membership.Workspace.ID, Status: in.Status, Search: in.Search,
			RowLimit: int64(max(min(in.Limit, 100), 1)), RowOffset: int64(max(in.Offset, 0)),
		})

		return wrap("list series", err)
	})

	return out, err
}

/*
Show returns one series.
*/
func (s *Series) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.Series, error) {
	if !m.Can("series:read") {
		return models.Series{}, exceptions.ErrForbidden
	}

	var out models.Series

	err := s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = find(ctx, q, m.Workspace.ID, id)

		return err
	})

	return out, err
}

/*
CreateInput starts a draft: its name, whether contacts or companies
travel it, and the definition to start from, an empty one when nil.
*/
type CreateInput struct {
	Membership authz.Membership
	Name       string
	Subject    string
	Spec       *Spec
}

/*
Create starts a draft for contacts or companies, sent by the teammate
who made it.
*/
func (s *Series) Create(ctx context.Context, in CreateInput) (models.Series, error) {
	m, name, subject, spec := in.Membership, in.Name, in.Subject, in.Spec
	if !m.Can("series:write") {
		return models.Series{}, exceptions.ErrForbidden
	}

	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 120 {
		return models.Series{}, ErrName
	}

	if subject != SubjectContact && subject != SubjectCompany {
		return models.Series{}, ErrSubject
	}

	start := NewSpec()
	if spec != nil {
		start = *spec
	}

	working, hash, err := encode(start)
	if err != nil {
		return models.Series{}, err
	}

	var out models.Series

	err = s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		var err error

		out, err = q.CreateSeries(ctx, models.CreateSeriesParams{
			WorkspaceID: m.Workspace.ID, Name: name, Subject: subject, Working: working, WorkingHash: hash,
			MemberID: &m.Member.ID, Now: s.Clock.Now(),
		})

		return nameTaken(err, "create series")
	})

	return out, err
}

/*
Delete removes a draft that never went live.
*/
func (s *Series) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("series:write") {
		return exceptions.ErrForbidden
	}

	return s.inWorkspace(ctx, m.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		n, err := q.DeleteSeries(ctx, models.DeleteSeriesParams{WorkspaceID: m.Workspace.ID, ID: id})
		if err != nil {
			return wrap("delete series", err)
		}

		if n == 0 {
			return ErrNotDraft
		}

		return nil
	})
}

/*
Duplicate copies a series' working copy and settings into a new draft.
*/
func (s *Series) Duplicate(ctx context.Context, m authz.Membership, id ids.UUID) (models.Series, error) {
	row, err := s.Show(ctx, m, id)
	if err != nil {
		return models.Series{}, err
	}

	spec, err := SpecOf(row)
	if err != nil {
		return models.Series{}, err
	}

	name := []rune("Copy of " + row.Name)

	out, err := s.Create(ctx, CreateInput{
		Membership: m, Name: string(name[:min(len(name), 120)]), Subject: row.Subject, Spec: &spec,
	})
	if err != nil {
		return out, err
	}

	in := CurrentSettings(row)
	in.Membership, in.ID, in.LockVersion, in.Name = m, out.ID, out.LockVersion, out.Name

	return s.Save(ctx, in)
}

/*
SpecOf reads a series' working copy.
*/
func SpecOf(row models.Series) (Spec, error) {
	return decode(row.Working)
}

/*
decode reads a stored spec.
*/
func decode(raw json.RawMessage) (Spec, error) {
	if len(raw) == 0 || string(raw) == "{}" {
		return NewSpec(), nil
	}

	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return NewSpec(), fmt.Errorf("read series spec: %w", err)
	}

	return Normalize(spec), nil
}

/*
Normalize fills in what a spec read from JSON may leave out: the entry
step, an empty list of paths, and entering once.
*/
func Normalize(spec Spec) Spec {
	if len(spec.Nodes) == 0 {
		spec.Nodes = NewSpec().Nodes
	}

	if spec.Edges == nil {
		spec.Edges = []Edge{}
	}

	if spec.Entry.Reentry.Kind == "" {
		spec.Entry.Reentry.Kind = "once"
	}

	return spec
}

/*
encode stores a spec and returns its hash. The hash is taken over the
JSON with its keys in order, because the database hands JSON back with
its keys reordered and the same spec must always hash the same.
*/
func encode(spec Spec) (json.RawMessage, string, error) {
	if spec.Edges == nil {
		spec.Edges = []Edge{}
	}

	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, "", fmt.Errorf("write series spec: %w", err)
	}

	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, "", fmt.Errorf("read series spec: %w", err)
	}

	canonical, err := json.Marshal(tree)
	if err != nil {
		return nil, "", fmt.Errorf("write series spec: %w", err)
	}

	return raw, hashing.SHA256Hex(canonical), nil
}

/*
inWorkspace runs fn in a transaction scoped to the workspace.
*/
func (s *Series) inWorkspace(
	ctx context.Context, ws ids.UUID, fn func(context.Context, *models.Queries) error,
) error {
	return s.DB.Transaction(withTenant(ctx, ws), func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
}

/*
inTx runs fn in a transaction scoped to the workspace with the
transaction itself, for work that queues jobs or sends messages.
*/
func (s *Series) inTx(ctx context.Context, ws ids.UUID, fn func(context.Context, *database.Tx) error) error {
	return s.DB.Transaction(withTenant(ctx, ws), fn)
}

/*
withTenant scopes the context to the workspace.
*/
func withTenant(ctx context.Context, ws ids.UUID) context.Context {
	return tenancy.With(ctx, ws.String())
}

/*
find returns one series, or not found.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Series, error) {
	row, err := q.FindSeries(ctx, models.FindSeriesParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return row, exceptions.ErrNotFound
	}

	return row, wrap("find series", err)
}

/*
nameTaken turns the unique name index into its own error.
*/
func nameTaken(err error, what string) error {
	if database.IsUniqueViolation(err, "series_workspace_id_name_key") {
		return ErrNameTaken
	}

	return wrap(what, err)
}

/*
wrap names what was being attempted.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}

/*
LiveHash returns the hash of the version a series runs, "" when it never
went live, so a caller can tell whether the working copy changed since.
*/
func (s *Series) LiveHash(ctx context.Context, row models.Series) string {
	if row.LiveVersionID == nil {
		return ""
	}

	hash := ""

	_ = s.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		v, err := q.FindSeriesVersion(ctx, models.FindSeriesVersionParams{
			WorkspaceID: row.WorkspaceID,
			ID:          *row.LiveVersionID,
		})
		hash = v.SpecHash

		return err
	})

	return hash
}

/*
Totals counts a series' journeys for the list: entered, on it now,
finished, left and reached the goal.
*/
func (s *Series) Totals(ctx context.Context, row models.Series) models.SeriesTotalsRow {
	var out models.SeriesTotalsRow

	_ = s.inWorkspace(ctx, row.WorkspaceID, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.SeriesTotals(ctx, models.SeriesTotalsParams{WorkspaceID: row.WorkspaceID, SeriesID: row.ID})

		return err
	})

	return out
}
