/*
|--------------------------------------------------------------------------
| Member Notification Settings
|--------------------------------------------------------------------------
|
| What a member chose: which channels each trigger reaches them on, and
| when not to disturb them at all. A member who never opened the settings
| has no rows and gets the defaults. Quiet hours run on the member's own
| clock, in their time zone, and a window that crosses midnight belongs
| to the day it started, so "Fridays 22:00 to 08:00" covers Saturday
| morning too.
|
*/

package notifications

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Choices are a member's settings and the cells they changed.
*/
type Choices struct {
	Settings models.NotificationSetting
	Cells    map[string]map[string]bool
}

/*
LoadChoices returns a member's choices, the defaults filled in.
*/
func LoadChoices(ctx context.Context, q *models.Queries, ws, member ids.UUID) (Choices, error) {
	out := Choices{Settings: DefaultSettings(ws, member), Cells: map[string]map[string]bool{}}
	row, err := q.FindNotificationSettings(ctx, models.FindNotificationSettingsParams{
		WorkspaceID: ws, WorkspaceMemberID: member,
	})
	if err == nil {
		out.Settings = row
	}
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return out, fmt.Errorf("load notification settings: %w", err)
	}
	prefs, err := q.ListNotificationPreferences(ctx, models.ListNotificationPreferencesParams{
		WorkspaceID: ws, WorkspaceMemberID: member,
	})
	if err != nil {
		return out, fmt.Errorf("load notification preferences: %w", err)
	}
	for _, p := range prefs {
		if out.Cells[p.Trigger] == nil {
			out.Cells[p.Trigger] = map[string]bool{}
		}
		out.Cells[p.Trigger][p.Channel] = p.Enabled
	}
	return out, nil
}

/*
DefaultSettings are the settings of a member who changed none.
*/
func DefaultSettings(ws, member ids.UUID) models.NotificationSetting {
	return models.NotificationSetting{
		WorkspaceID: ws, WorkspaceMemberID: member, QuietHoursStart: "22:00", QuietHoursEnd: "08:00",
		QuietHoursDays: 127, QuietHoursBypass: []string{}, ShowPreviews: true,
	}
}

/*
Enabled reports whether the trigger reaches the member on the channel.
*/
func (c Choices) Enabled(t Trigger, channel string) bool {
	if t.IsLocked(channel) {
		return true
	}
	if v, ok := c.Cells[t.Key][channel]; ok {
		return v
	}
	return t.Default(channel)
}

/*
Quiet reports whether the member asked not to be disturbed now about
the trigger: paused, or inside quiet hours it does not bypass.
*/
func (c Choices) Quiet(now time.Time, trigger, fallbackZone string) bool {
	return c.QuietReason(now, trigger, fallbackZone) != ""
}

/*
QuietReason says why the member is not to be disturbed now about the
trigger: paused, or quiet hours; "" when they may be.
*/
func (c Choices) QuietReason(now time.Time, trigger, fallbackZone string) string {
	s := c.Settings
	if s.PausedUntil != nil && now.Before(*s.PausedUntil) {
		return ReasonPaused
	}
	if !s.QuietHoursEnabled || slices.Contains(s.QuietHoursBypass, trigger) {
		return ""
	}
	if InQuietHours(now, c.Window(fallbackZone)) {
		return ReasonQuietHours
	}
	return ""
}

/*
Window is the member's quiet window, in their zone or else the one
given.
*/
func (c Choices) Window(fallbackZone string) Window {
	s := c.Settings
	zone := deref(s.Timezone)
	if zone == "" {
		zone = fallbackZone
	}
	return Window{Zone: zone, Start: s.QuietHoursStart, End: s.QuietHoursEnd, Days: s.QuietHoursDays}
}

/*
Window is a nightly quiet window: wall clock "HH:MM" start and end in a
time zone, on the days the mask names (Monday 1 through Sunday 64).
*/
type Window struct {
	Zone  string
	Start string
	End   string
	Days  int16
}

/*
InQuietHours reports whether the moment falls in the window. A window
whose end is before its start runs past midnight and counts for the day
it started.
*/
func InQuietHours(now time.Time, w Window) bool {
	loc, err := time.LoadLocation(w.Zone)
	if err != nil || w.Zone == "" {
		loc = time.UTC
	}
	local := now.In(loc)
	days := w.Days
	from, okFrom := minutes(w.Start)
	to, okTo := minutes(w.End)
	if !okFrom || !okTo || from == to {
		return false
	}
	at := local.Hour()*60 + local.Minute()
	if from < to {
		return at >= from && at < to && dayIn(days, local.Weekday())
	}
	if at >= from {
		return dayIn(days, local.Weekday())
	}
	return at < to && dayIn(days, local.AddDate(0, 0, -1).Weekday())
}

/*
dayIn reports whether the mask names the weekday.
*/
func dayIn(days int16, d time.Weekday) bool {
	bit := (int(d) + 6) % 7
	return days&(1<<bit) != 0
}

/*
minutes reads "HH:MM" as minutes after midnight.
*/
func minutes(hhmm string) (int, bool) {
	h, m, ok := strings.Cut(hhmm, ":")
	if !ok {
		return 0, false
	}
	hours, err1 := strconv.Atoi(h)
	mins, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hours < 0 || hours > 23 || mins < 0 || mins > 59 {
		return 0, false
	}
	return hours*60 + mins, true
}
