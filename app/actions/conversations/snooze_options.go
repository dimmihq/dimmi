/*
|--------------------------------------------------------------------------
| Snooze Options
|--------------------------------------------------------------------------
|
| The choices a teammate picks a snooze from. A workspace may set its own
| list and each team its own for the conversations it holds; without
| one, the built-in presets apply. An option says when the snooze ends in
| the reader's own time zone: a preset, a number of minutes from now, a
| day ahead at a time, or the next given weekday at a time, never today.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The kinds of snooze option, and the most options a list holds.
*/
const (
	OptionPreset    = "preset"
	OptionIn        = "in"
	OptionDayAt     = "day_at"
	OptionWeekdayAt = "weekday_at"

	maxSnoozeOptions = 10
)

/*
SnoozeOption is one choice: its key, label, kind, and the fields its
kind reads. Preset names a built-in preset; Minutes counts from now;
DaysAhead and At give a day ahead at a time ("09:00"); Weekday (1 for
Monday to 7 for Sunday) and At give the next such day at a time.
*/
type SnoozeOption struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Preset    string `json:"preset,omitempty"`
	Minutes   int    `json:"minutes,omitempty"`
	DaysAhead int    `json:"days_ahead,omitempty"`
	Weekday   int    `json:"weekday,omitempty"`
	At        string `json:"at,omitempty"`
}

/*
BuiltInSnoozeOptions returns the presets offered when no list is set.
*/
func BuiltInSnoozeOptions() []SnoozeOption {
	return []SnoozeOption{
		{Key: PresetLaterToday, Label: "Later today", Kind: OptionPreset, Preset: PresetLaterToday},
		{Key: PresetTomorrow, Label: "Tomorrow", Kind: OptionPreset, Preset: PresetTomorrow},
		{Key: PresetNextWeek, Label: "Next week", Kind: OptionPreset, Preset: PresetNextWeek},
		{Key: PresetOneWeek, Label: "In one week", Kind: OptionPreset, Preset: PresetOneWeek},
	}
}

/*
clockTime is what an option's At looks like.
*/
var clockTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

/*
ValidateSnoozeOptions refuses an empty list, more than ten options, a
repeated key and an option its kind cannot resolve.
*/
func ValidateSnoozeOptions(options []SnoozeOption) error {
	if len(options) == 0 || len(options) > maxSnoozeOptions {
		return exceptions.ErrInvalidSnoozeOptions.With("reason", "between 1 and 10 options")
	}

	seen := map[string]bool{}

	for i, o := range options {
		if o.Key == "" || len(o.Key) > 40 || o.Label == "" || len(o.Label) > 60 || seen[o.Key] {
			return exceptions.ErrInvalidSnoozeOptions.With("index", i)
		}

		seen[o.Key] = true

		if !validOption(o) {
			return exceptions.ErrInvalidSnoozeOptions.With("index", i)
		}
	}

	return nil
}

/*
validOption reports whether the option's kind has the fields it needs.
*/
func validOption(o SnoozeOption) bool {
	switch o.Kind {
	case OptionPreset:
		return slices.Contains([]string{PresetLaterToday, PresetTomorrow, PresetNextWeek, PresetOneWeek}, o.Preset)
	case OptionIn:
		return o.Minutes >= 5 && o.Minutes <= 525600
	case OptionDayAt:
		return o.DaysAhead >= 1 && o.DaysAhead <= 365 && clockTime.MatchString(o.At)
	case OptionWeekdayAt:
		return o.Weekday >= 1 && o.Weekday <= 7 && clockTime.MatchString(o.At)
	}

	return false
}

/*
ResolveSnoozeOption returns when the option ends, from now in the
reader's location.
*/
func ResolveSnoozeOption(o SnoozeOption, now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)

	switch o.Kind {
	case OptionIn:
		return now.Add(time.Duration(o.Minutes) * time.Minute)
	case OptionDayAt:
		return atClock(local.AddDate(0, 0, o.DaysAhead), o.At)
	case OptionWeekdayAt:
		ahead := (o.Weekday%week - int(local.Weekday()) + week) % week
		if ahead == 0 {
			ahead = week
		}

		return atClock(local.AddDate(0, 0, ahead), o.At)
	}

	return resolvePreset(local, o.Preset)
}

/*
atClock returns the day at the hour and minute, in the day's location.
*/
func atClock(day time.Time, at string) time.Time {
	t, err := time.Parse("15:04", at)
	if err != nil {
		return day
	}

	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, day.Location())
}

/*
snoozeOptionsFor returns the options that apply to a conversation the
team holds: the team's list, the workspace's, or the built-in presets.
*/
func snoozeOptionsFor(ctx context.Context, q *models.Queries, workspace ids.UUID, team *ids.UUID) ([]SnoozeOption, error) {
	rows, err := q.SnoozeListsFor(ctx, models.SnoozeListsForParams{WorkspaceID: workspace, TeamID: team})
	if err != nil {
		return nil, fmt.Errorf("load snooze lists: %w", err)
	}

	for _, row := range rows {
		var options []SnoozeOption
		if json.Unmarshal(row.Options, &options) == nil && len(options) > 0 {
			return options, nil
		}
	}

	return BuiltInSnoozeOptions(), nil
}

/*
snoozeOption finds the option of the key among those that apply,
refusing a key no longer offered.
*/
func snoozeOption(
	ctx context.Context, q *models.Queries, c models.Conversation, key string,
) (SnoozeOption, error) {
	options, err := snoozeOptionsFor(ctx, q, c.WorkspaceID, c.AssigneeTeamID)
	if err != nil {
		return SnoozeOption{}, err
	}

	for _, o := range options {
		if o.Key == key {
			return o, nil
		}
	}

	return SnoozeOption{}, exceptions.ErrSnoozeOptionUnknown
}
