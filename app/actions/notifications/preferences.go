/*
|--------------------------------------------------------------------------
| Notification Preferences
|--------------------------------------------------------------------------
|
| The settings page's matrix: every trigger a member can receive, by
| channel, with the default and whether it is locked. Turning a cell back
| to its default forgets it, so a later change of default reaches them.
| Settings hold quiet hours, a pause, previews and whether to hear about
| conversations the agent is answering; a time zone left empty follows
| the member's account.
|
*/

package notifications

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxPause is the longest a member can pause notifications for.
*/
const maxPause = 30 * 24 * time.Hour

/*
Cell is one trigger on one channel: whether it is on, its default and
whether it can be turned off.
*/
type Cell struct {
	Trigger string
	Channel string
	Enabled bool
	Default bool
	Locked  bool
}

/*
Matrix is a member's whole notification setup.
*/
type Matrix struct {
	Triggers []Trigger
	Cells    []Cell
	Settings models.NotificationSetting
}

/*
CellChange turns one cell on or off.
*/
type CellChange struct {
	Trigger string
	Channel string
	Enabled bool
}

/*
SettingsChange is a change to a member's settings; nil leaves a field
as it is.
*/
type SettingsChange struct {
	Timezone          *string
	QuietHoursEnabled *bool
	QuietHoursStart   *string
	QuietHoursEnd     *string
	QuietHoursDays    *int16
	QuietHoursBypass  *[]string
	PausedUntil       *time.Time
	ClearPause        bool
	ShowPreviews      *bool
	IncludeAIHandled  *bool
}

/*
ErrPreferenceLocked refuses turning off a channel that must stay on.
*/
var ErrPreferenceLocked = fault.New(fault.Invalid, "preference_locked", "That notification cannot be turned off.")

/*
Preferences returns the member's matrix.
*/
func (c *Center) Preferences(ctx context.Context, m authz.Membership) (Matrix, error) {
	if err := requireRead(m); err != nil {
		return Matrix{}, err
	}
	var out Matrix
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		choices, err := LoadChoices(ctx, models.New(tx), m.Workspace.ID, m.Member.ID)
		out = matrixOf(m, choices)
		return err
	})
	return out, err
}

/*
matrixOf lays the member's choices over every trigger they may receive.
*/
func matrixOf(m authz.Membership, choices Choices) Matrix {
	out := Matrix{Settings: choices.Settings}
	for _, t := range Triggers() {
		if t.Requires != "" && !m.Can(t.Requires) {
			continue
		}
		out.Triggers = append(out.Triggers, t)
		for _, ch := range Channels() {
			out.Cells = append(out.Cells, Cell{
				Trigger: t.Key, Channel: ch, Enabled: choices.Enabled(t, ch), Default: t.Default(ch),
				Locked: t.IsLocked(ch),
			})
		}
	}
	return out
}

/*
UpdatePreferences changes cells and returns the matrix.
*/
func (c *Center) UpdatePreferences(ctx context.Context, m authz.Membership, changes []CellChange) (Matrix, error) {
	if err := requireWrite(m); err != nil {
		return Matrix{}, err
	}
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		for _, ch := range changes {
			if err := c.setCell(ctx, q, m, ch); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Matrix{}, err
	}
	return c.Preferences(ctx, m)
}

/*
setCell stores one cell, or forgets it when it matches the default.
*/
func (c *Center) setCell(ctx context.Context, q *models.Queries, m authz.Membership, ch CellChange) error {
	t, ok := FindTrigger(ch.Trigger)
	if !ok || !slices.Contains(Channels(), ch.Channel) {
		return fault.New(fault.Invalid, "preference_unknown", "That trigger or channel does not exist.")
	}
	if t.IsLocked(ch.Channel) && !ch.Enabled {
		return ErrPreferenceLocked
	}
	if ch.Enabled == t.Default(ch.Channel) {
		return q.DeleteNotificationPreference(ctx, models.DeleteNotificationPreferenceParams{
			WorkspaceID: m.Workspace.ID, WorkspaceMemberID: m.Member.ID, Trigger: ch.Trigger, Channel: ch.Channel,
		})
	}
	return q.UpsertNotificationPreference(ctx, models.UpsertNotificationPreferenceParams{
		WorkspaceID: m.Workspace.ID, WorkspaceMemberID: m.Member.ID, Trigger: ch.Trigger, Channel: ch.Channel,
		Enabled: ch.Enabled, Now: c.Clock.Now(),
	})
}

/*
ResetPreferences puts every cell back to its default.
*/
func (c *Center) ResetPreferences(ctx context.Context, m authz.Membership) (Matrix, error) {
	if err := requireWrite(m); err != nil {
		return Matrix{}, err
	}
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).DeleteNotificationPreferences(ctx, models.DeleteNotificationPreferencesParams{
			WorkspaceID: m.Workspace.ID, WorkspaceMemberID: m.Member.ID,
		})
	})
	if err != nil {
		return Matrix{}, fmt.Errorf("reset notification preferences: %w", err)
	}
	return c.Preferences(ctx, m)
}

/*
UpdateSettings changes the member's settings and returns them.
*/
func (c *Center) UpdateSettings(ctx context.Context, m authz.Membership, in SettingsChange,
) (models.NotificationSetting, error) {
	if err := requireWrite(m); err != nil {
		return models.NotificationSetting{}, err
	}
	var out models.NotificationSetting
	err := c.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		choices, err := LoadChoices(ctx, q, m.Workspace.ID, m.Member.ID)
		if err != nil {
			return err
		}
		next, err := c.apply(choices.Settings, in)
		if err != nil {
			return err
		}
		out, err = q.UpsertNotificationSettings(ctx, upsertParams(next, c.Clock.Now()))
		return err
	})
	return out, err
}

/*
apply checks and writes the change over the settings.
*/
func (c *Center) apply(s models.NotificationSetting, in SettingsChange) (models.NotificationSetting, error) {
	if in.Timezone != nil {
		if *in.Timezone != "" {
			if _, err := time.LoadLocation(*in.Timezone); err != nil || *in.Timezone == "Local" {
				return s, invalid("timezone", "Pick a time zone such as Europe/Paris.")
			}
		}
		s.Timezone = in.Timezone
	}
	if err := applyQuietHours(&s, in); err != nil {
		return s, err
	}
	if in.ClearPause {
		s.PausedUntil = nil
	}
	if in.PausedUntil != nil {
		if in.PausedUntil.After(c.Clock.Now().Add(maxPause)) {
			return s, invalid("paused_until", "Pause for at most 30 days.")
		}
		s.PausedUntil = in.PausedUntil
	}
	if in.ShowPreviews != nil {
		s.ShowPreviews = *in.ShowPreviews
	}
	if in.IncludeAIHandled != nil {
		s.IncludeAiHandled = *in.IncludeAIHandled
	}
	return s, nil
}

/*
applyQuietHours writes the quiet-hours part of a change.
*/
func applyQuietHours(s *models.NotificationSetting, in SettingsChange) error {
	if in.QuietHoursEnabled != nil {
		s.QuietHoursEnabled = *in.QuietHoursEnabled
	}
	if err := setClock(&s.QuietHoursStart, in.QuietHoursStart, "quiet_hours_start"); err != nil {
		return err
	}
	if err := setClock(&s.QuietHoursEnd, in.QuietHoursEnd, "quiet_hours_end"); err != nil {
		return err
	}
	if in.QuietHoursDays != nil {
		if *in.QuietHoursDays < 0 || *in.QuietHoursDays > 127 {
			return invalid("quiet_hours_days", "Pick days of the week.")
		}
		s.QuietHoursDays = *in.QuietHoursDays
	}
	return setBypass(s, in.QuietHoursBypass)
}

/*
setClock writes a sent "HH:MM" time over the field.
*/
func setClock(field, sent *string, name string) error {
	if sent == nil {
		return nil
	}
	if _, ok := minutes(*sent); !ok || len(*sent) != 5 {
		return invalid(name, "Use a time such as 22:00.")
	}
	*field = *sent
	return nil
}

/*
setBypass writes the triggers that come through quiet hours.
*/
func setBypass(s *models.NotificationSetting, sent *[]string) error {
	if sent == nil {
		return nil
	}
	for _, key := range *sent {
		if _, ok := FindTrigger(key); !ok {
			return invalid("quiet_hours_bypass", "That trigger does not exist.")
		}
	}
	s.QuietHoursBypass = *sent
	return nil
}

func upsertParams(s models.NotificationSetting, now time.Time) models.UpsertNotificationSettingsParams {
	return models.UpsertNotificationSettingsParams{
		WorkspaceMemberID: s.WorkspaceMemberID, WorkspaceID: s.WorkspaceID, Timezone: s.Timezone,
		QuietHoursEnabled: s.QuietHoursEnabled, QuietHoursStart: s.QuietHoursStart, QuietHoursEnd: s.QuietHoursEnd,
		QuietHoursDays: s.QuietHoursDays, QuietHoursBypass: s.QuietHoursBypass, PausedUntil: s.PausedUntil,
		ShowPreviews: s.ShowPreviews, IncludeAiHandled: s.IncludeAiHandled, Now: now,
	}
}

func invalid(field, message string) error {
	return fault.New(fault.Invalid, field+"_invalid", message).OnField(field)
}
