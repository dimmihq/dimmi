/*
|--------------------------------------------------------------------------
| Inbox Office Hours
|--------------------------------------------------------------------------
|
| Every inbox answers "is the team around" from one schedule: its own,
| or the workspace's default when it has none. The agent, the Slack
| auto-reply and workflows that name no schedule all ask through here,
| so an inbox's hours are set in one place and read the same way
| everywhere.
|
| An inbox takes its own hours as a copy of the default, named after
| it, and gives them up by following the default again; its own
| schedule is then removed unless a workflow still names it.
|
*/

package schedules

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxName is the longest a schedule's name may be.
*/
const maxName = 60

/*
Place is an inbox of a workspace whose office hours are asked for.
*/
type Place struct {
	Workspace, Inbox ids.UUID
}

/*
ForInbox returns the schedule an inbox keeps its office hours in: its
own, or the workspace's default, made on first use, when it has none.
*/
func ForInbox(ctx context.Context, q *models.Queries, p Place, c clock.Clock) (models.Schedule, error) {
	own, err := q.FindInboxSchedule(ctx, models.FindInboxScheduleParams{WorkspaceID: p.Workspace, InboxID: p.Inbox})
	if err == nil {
		return own, nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return own, fmt.Errorf("find inbox schedule: %w", err)
	}

	return Default(ctx, q, p.Workspace, c)
}

/*
OpenAt reports whether an inbox's office hours are open at the moment.
Hours that cannot be read answer false with the error, so a caller
that only looks at the answer fails closed.
*/
func OpenAt(ctx context.Context, q *models.Queries, p Place, c clock.Clock) (bool, error) {
	row, err := ForInbox(ctx, q, p, c)
	if err != nil {
		return false, err
	}

	sched, err := Read(row)
	if err != nil {
		return false, err
	}

	return sched.IsOpen(c.Now()), nil
}

/*
InboxHoursInput is who changes which inbox's hours: Own true gives it
hours of its own, false makes it follow the workspace's default.
*/
type InboxHoursInput struct {
	Membership authz.Membership
	InboxID    ids.UUID
	Own        bool
}

/*
SetInboxHours gives an inbox hours of its own or makes it follow the
workspace's default, and returns the schedule it now uses. Asking for
what the inbox already has changes nothing. It takes both
inboxes:manage and schedules:manage.
*/
func (s *Schedules) SetInboxHours(ctx context.Context, in InboxHoursInput) (models.Schedule, error) {
	if !in.Membership.Can("inboxes:manage") || !in.Membership.Can("schedules:manage") {
		return models.Schedule{}, exceptions.ErrForbidden
	}

	var out models.Schedule
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID
		inbox, err := q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: ws, ID: in.InboxID})
		if errors.Is(err, database.ErrNotFound) || (err == nil && inbox.DeletedAt != nil) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find inbox: %w", err)
		}

		switch {
		case in.Own && inbox.ScheduleID == nil:
			out, err = s.ownHours(ctx, q, inbox)
		case !in.Own && inbox.ScheduleID != nil:
			out, err = s.followDefault(ctx, q, inbox)
		default:
			out, err = ForInbox(ctx, q, Place{Workspace: ws, Inbox: inbox.ID}, s.Clock)
		}

		return err
	})

	return out, err
}

/*
ownHours copies the default into a schedule named after the inbox and
points the inbox at it.
*/
func (s *Schedules) ownHours(ctx context.Context, q *models.Queries, inbox models.Inbox) (models.Schedule, error) {
	def, err := Default(ctx, q, inbox.WorkspaceID, s.Clock)
	if err != nil {
		return def, err
	}

	name, err := freeName(ctx, q, inbox.WorkspaceID, inbox.Name+" hours")
	if err != nil {
		return def, err
	}

	now := s.Clock.Now()
	own, err := q.CreateSchedule(ctx, models.CreateScheduleParams{
		ID: ids.NewV7At(now), WorkspaceID: inbox.WorkspaceID, Name: name, Timezone: def.Timezone,
		Weekly: def.Weekly, Holidays: def.Holidays, Now: now,
	})
	if err != nil {
		return own, fmt.Errorf("create inbox schedule: %w", err)
	}

	return own, point(ctx, q, models.SetInboxScheduleParams{
		WorkspaceID: inbox.WorkspaceID, ID: inbox.ID, ScheduleID: &own.ID, Now: now,
	})
}

/*
followDefault points the inbox back at the default and removes the
schedule it had, unless it is the default or something still uses it.
*/
func (s *Schedules) followDefault(ctx context.Context, q *models.Queries, inbox models.Inbox) (models.Schedule, error) {
	now, old := s.Clock.Now(), *inbox.ScheduleID
	if err := point(ctx, q, models.SetInboxScheduleParams{
		WorkspaceID: inbox.WorkspaceID, ID: inbox.ID, Now: now,
	}); err != nil {
		return models.Schedule{}, err
	}

	def, err := Default(ctx, q, inbox.WorkspaceID, s.Clock)
	if err != nil || def.ID == old {
		return def, err
	}

	used, err := q.ScheduleInUse(ctx, models.ScheduleInUseParams{
		WorkspaceID: inbox.WorkspaceID, ID: old, PublicID: models.PublicID(models.PrefixSchedule, old),
	})
	if err != nil {
		return def, fmt.Errorf("check schedule use: %w", err)
	}

	if used {
		return def, nil
	}

	err = q.DeleteSchedule(ctx, models.DeleteScheduleParams{WorkspaceID: inbox.WorkspaceID, ID: old, Now: new(now)})

	return def, wrap("delete inbox schedule", err)
}

/*
point names the schedule an inbox keeps its hours in, nil for the
default.
*/
func point(ctx context.Context, q *models.Queries, p models.SetInboxScheduleParams) error {
	_, err := q.SetInboxSchedule(ctx, p)

	return wrap("set inbox schedule", err)
}

/*
freeName returns the name, cut to fit, or the name with the first
number that makes it unique among the workspace's schedules.
*/
func freeName(ctx context.Context, q *models.Queries, workspace ids.UUID, base string) (string, error) {
	base = clip(base, maxName-3)
	for n := 1; ; n++ {
		name := base
		if n > 1 {
			name = base + " " + strconv.Itoa(n)
		}

		taken, err := q.ScheduleNameTaken(ctx, models.ScheduleNameTakenParams{WorkspaceID: workspace, Name: name})
		if err != nil {
			return "", fmt.Errorf("check schedule name: %w", err)
		}

		if !taken {
			return name, nil
		}
	}
}

/*
clip cuts a name to at most the given number of characters.
*/
func clip(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}

	return string([]rune(s)[:limit])
}
