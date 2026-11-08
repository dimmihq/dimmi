/*
|--------------------------------------------------------------------------
| Changing Schedules
|--------------------------------------------------------------------------
|
| A schedule is checked whole before it is stored: a timezone the
| server knows, ranges that end after they start and never overlap, at
| most four a day, and real dates for holidays. Making a schedule the
| default takes the mark off the old one in the same transaction, so a
| workspace always has exactly one. Waits already counting on a
| schedule keep the wake time they were given; new waits use the hours
| as they now are.
|
*/

package schedules

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
stored is a schedule's checked week and holidays as JSON.
*/
type stored struct {
	weekly, holidays json.RawMessage
}

/*
check validates the schedule's hours and encodes them.
*/
func check(timezone string, weekly officehours.Weekly, holidays []officehours.Holiday) (stored, error) {
	if weekly == nil {
		weekly = officehours.Weekly{}
	}

	if holidays == nil {
		holidays = []officehours.Holiday{}
	}

	if _, err := officehours.ParseSchedule(timezone, weekly, holidays); err != nil {
		return stored{}, ErrInvalid.With("reason", err.Error())
	}

	w, _ := json.Marshal(weekly)
	h, _ := json.Marshal(holidays)

	return stored{weekly: w, holidays: h}, nil
}

/*
Create stores a new schedule.
*/
func (s *Schedules) Create(ctx context.Context, in Input) (models.Schedule, error) {
	if !in.Membership.Can("schedules:manage") {
		return models.Schedule{}, exceptions.ErrForbidden
	}

	name, err := cleanName(deref(in.Name))
	if err != nil {
		return models.Schedule{}, err
	}

	hours, err := check(deref(in.Timezone), in.Weekly, in.Holidays)
	if err != nil {
		return models.Schedule{}, err
	}

	ws, now := in.Membership.Workspace.ID, s.Clock.Now()
	var out models.Schedule
	err = s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		free := models.ScheduleNameTakenParams{WorkspaceID: ws, Name: name, ID: ids.Nil()}
		if err := nameFree(ctx, q, free); err != nil {
			return err
		}

		out, err = q.CreateSchedule(ctx, models.CreateScheduleParams{
			ID: ids.NewV7At(now), WorkspaceID: ws, Name: name, Timezone: deref(in.Timezone), Weekly: hours.weekly,
			Holidays: hours.holidays, Now: now,
		})
		if err != nil {
			return fmt.Errorf("create schedule: %w", err)
		}

		if !in.Default {
			return nil
		}

		return s.markDefault(ctx, q, &out)
	})

	return out, err
}

/*
Update changes a schedule's name, hours or holidays, or makes it the
default.
*/
func (s *Schedules) Update(ctx context.Context, in Input) (models.Schedule, error) {
	if !in.Membership.Can("schedules:manage") {
		return models.Schedule{}, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace.ID
	var out models.Schedule
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := find(ctx, q, ws, in.ID)
		if err != nil {
			return err
		}

		params, err := s.changed(ctx, q, current, in)
		if err != nil {
			return err
		}

		if out, err = q.UpdateSchedule(ctx, params); err != nil {
			return fmt.Errorf("update schedule: %w", err)
		}

		if !in.Default {
			return nil
		}

		return s.markDefault(ctx, q, &out)
	})

	return out, err
}

/*
changed is the schedule's row with the input's changes, checked.
*/
func (s *Schedules) changed(ctx context.Context, q *models.Queries, current models.Schedule, in Input,
) (models.UpdateScheduleParams, error) {
	p := models.UpdateScheduleParams{
		Name: current.Name, Timezone: current.Timezone, Weekly: current.Weekly, Holidays: current.Holidays,
		Now: s.Clock.Now(), WorkspaceID: current.WorkspaceID, ID: current.ID,
	}

	if in.Name != nil {
		name, err := cleanName(*in.Name)
		if err != nil {
			return p, err
		}

		free := models.ScheduleNameTakenParams{WorkspaceID: current.WorkspaceID, Name: name, ID: current.ID}
		if err := nameFree(ctx, q, free); err != nil {
			return p, err
		}

		p.Name = name
	}

	return withHours(p, current, in)
}

/*
withHours applies changed hours, keeping the week and holidays not
given.
*/
func withHours(p models.UpdateScheduleParams, current models.Schedule, in Input) (models.UpdateScheduleParams, error) {
	if in.Timezone == nil && in.Weekly == nil && in.Holidays == nil {
		return p, nil
	}

	parsed, err := Read(current)
	if err != nil {
		return p, ErrInvalid
	}

	weekly, holidays := in.Weekly, in.Holidays
	if weekly == nil {
		_ = json.Unmarshal(current.Weekly, &weekly)
	}

	if holidays == nil {
		_ = json.Unmarshal(current.Holidays, &holidays)
	}

	tz := parsed.Location().String()
	if in.Timezone != nil {
		tz = *in.Timezone
	}

	hours, err := check(tz, weekly, holidays)
	p.Timezone, p.Weekly, p.Holidays = tz, hours.weekly, hours.holidays

	return p, err
}

/*
markDefault makes the schedule the default, taking the mark off the
old default.
*/
func (s *Schedules) markDefault(ctx context.Context, q *models.Queries, sch *models.Schedule) error {
	if sch.IsDefault {
		return nil
	}

	now := s.Clock.Now()
	if err := q.ClearDefaultSchedule(ctx, models.ClearDefaultScheduleParams{
		WorkspaceID: sch.WorkspaceID, Now: now,
	}); err != nil {
		return fmt.Errorf("clear default schedule: %w", err)
	}

	if err := q.MarkDefaultSchedule(ctx, models.MarkDefaultScheduleParams{
		WorkspaceID: sch.WorkspaceID, ID: sch.ID, Now: now,
	}); err != nil {
		return fmt.Errorf("mark default schedule: %w", err)
	}

	sch.IsDefault = true

	return nil
}

/*
Delete removes a schedule nothing needs: not the default, and named by
no workflow.
*/
func (s *Schedules) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("schedules:manage") {
		return exceptions.ErrForbidden
	}

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		if current.IsDefault {
			return ErrDefault
		}

		used, err := q.ScheduleInUse(ctx, models.ScheduleInUseParams{
			WorkspaceID: m.Workspace.ID, PublicID: models.PublicID(models.PrefixSchedule, id),
		})
		if err != nil {
			return fmt.Errorf("check schedule use: %w", err)
		}

		if used {
			return ErrInUse
		}

		return wrap("delete schedule", q.DeleteSchedule(ctx, models.DeleteScheduleParams{
			WorkspaceID: m.Workspace.ID, ID: id, Now: new(s.Clock.Now()),
		}))
	})
}

/*
nameFree refuses a name another schedule has.
*/
func nameFree(ctx context.Context, q *models.Queries, p models.ScheduleNameTakenParams) error {
	taken, err := q.ScheduleNameTaken(ctx, p)
	if err != nil {
		return fmt.Errorf("check schedule name: %w", err)
	}

	if taken {
		return ErrNameTaken
	}

	return nil
}

/*
deref returns the text, empty for none.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
