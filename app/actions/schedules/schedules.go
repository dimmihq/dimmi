/*
|--------------------------------------------------------------------------
| Schedules
|--------------------------------------------------------------------------
|
| A workspace's office hours, named so workflows can ask whether the
| team is around, wait until it is, and count only open time. Every
| workspace has one default schedule, used wherever none is named; it
| is made on first use, open weekdays nine to five in the workspace's
| timezone, so a workflow never finds the team's hours missing. The
| default cannot be deleted, and neither can a schedule a workflow
| still names.
|
*/

package schedules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
The failures of schedules.
*/
var (
	ErrNotFound  = fault.New(fault.NotFound, "schedule_not_found", "No schedule of this workspace has this id.")
	ErrNameTaken = fault.New(fault.Conflict, "name_taken", "Another schedule already has this name.").
			OnField("name")
	ErrNameInvalid = fault.New(fault.Invalid, "name_invalid", "Names are 1 to 60 characters.").OnField("name")
	ErrInvalid     = fault.New(fault.Invalid, "schedule_invalid", "These office hours cannot be read.")
	ErrDefault     = fault.New(fault.Conflict, "schedule_default",
		"This is the default schedule. Make another the default first.")
	ErrInUse = fault.New(fault.Conflict, "schedule_in_use", "An inbox or a workflow still uses this schedule.")
)

/*
DefaultName is the name of the schedule every workspace starts with.
*/
const DefaultName = "Office hours"

/*
Schedules manages a workspace's schedules.
*/
type Schedules struct {
	DB    *database.DB
	Clock clock.Clock
	IDs   *ids.Registry
}

/*
Input is a schedule to store. Nil fields keep what is stored; Default
true makes the schedule the workspace's default.
*/
type Input struct {
	Membership authz.Membership
	ID         ids.UUID
	Name       *string
	Timezone   *string
	Weekly     officehours.Weekly
	Holidays   []officehours.Holiday
	Default    bool
}

/*
List returns the workspace's schedules, the default first, making the
default when the workspace has none yet.
*/
func (s *Schedules) List(ctx context.Context, m authz.Membership) ([]models.Schedule, error) {
	if !m.Can("conversations:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.Schedule
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := Default(ctx, q, m.Workspace.ID, s.Clock); err != nil {
			return err
		}

		var err error
		out, err = q.ListSchedules(ctx, m.Workspace.ID)

		return err
	})

	return out, err
}

/*
Show returns one schedule.
*/
func (s *Schedules) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.Schedule, error) {
	if !m.Can("conversations:read") {
		return models.Schedule{}, exceptions.ErrForbidden
	}

	var out models.Schedule
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = find(ctx, models.New(tx), m.Workspace.ID, id)

		return err
	})

	return out, err
}

/*
Default returns the workspace's default schedule, making it on first
use: weekdays nine to five in the workspace's timezone.
*/
func Default(ctx context.Context, q *models.Queries, workspace ids.UUID, c clock.Clock) (models.Schedule, error) {
	found, err := q.FindDefaultSchedule(ctx, workspace)
	if err == nil || !errors.Is(err, database.ErrNotFound) {
		return found, wrap("find default schedule", err)
	}

	ws, err := q.FindWorkspace(ctx, workspace)
	if err != nil {
		return found, fmt.Errorf("find workspace: %w", err)
	}

	day := [][2]string{{"09:00", "17:00"}}
	weekly, _ := json.Marshal(officehours.Weekly{"mon": day, "tue": day, "wed": day, "thu": day, "fri": day})
	tz := ws.Timezone
	if _, err := officehours.ParseSchedule(tz, nil, nil); err != nil {
		tz = "UTC"
	}

	now := c.Now()
	created, err := q.CreateSchedule(ctx, models.CreateScheduleParams{
		ID: ids.NewV7At(now), WorkspaceID: workspace, Name: DefaultName, Timezone: tz, Weekly: weekly,
		Holidays: json.RawMessage("[]"), IsDefault: true, Now: now,
	})

	return created, wrap("create default schedule", err)
}

/*
Read parses a stored schedule.
*/
func Read(s models.Schedule) (*officehours.Schedule, error) {
	parsed, err := officehours.ParseStored(s.Timezone, s.Weekly, s.Holidays)
	if err != nil {
		return nil, fmt.Errorf("read schedule %s: %w", s.ID, err)
	}

	return parsed, nil
}

/*
find returns one schedule of the workspace.
*/
func find(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Schedule, error) {
	found, err := q.FindSchedule(ctx, models.FindScheduleParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return found, ErrNotFound
	}

	return found, wrap("find schedule", err)
}

/*
wrap names what failed, leaving no error alone.
*/
func wrap(what string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", what, err)
}

/*
cleanName trims a name and checks its length.
*/
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		return "", ErrNameInvalid
	}

	return name, nil
}
