/*
|--------------------------------------------------------------------------
| Notification Settings Resources
|--------------------------------------------------------------------------
|
| A member's notification matrix, every trigger by channel with its
| default and lock, and their quiet hours, pause and choices.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/notifications"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NotificationTrigger is a row of the settings matrix.
*/
type NotificationTrigger struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

/*
NotificationCell is one trigger on one channel.
*/
type NotificationCell struct {
	Trigger string `json:"trigger"`
	Channel string `json:"channel"`
	Enabled bool   `json:"enabled"`
	Default bool   `json:"default"`
	Locked  bool   `json:"locked"`
}

/*
NotificationPreferences is a member's matrix with their settings.
*/
type NotificationPreferences struct {
	Triggers []NotificationTrigger `json:"triggers"`
	Channels []string              `json:"channels"`
	Cells    []NotificationCell    `json:"cells"`
	Settings NotificationSettings  `json:"settings"`
}

/*
NotificationSettings are a member's quiet hours, pause and choices.
*/
type NotificationSettings struct {
	Timezone          *string    `json:"timezone"`
	QuietHoursEnabled bool       `json:"quiet_hours_enabled"`
	QuietHoursStart   string     `json:"quiet_hours_start"`
	QuietHoursEnd     string     `json:"quiet_hours_end"`
	QuietHoursDays    int16      `json:"quiet_hours_days"`
	QuietHoursBypass  []string   `json:"quiet_hours_bypass"`
	PausedUntil       *time.Time `json:"paused_until"`
	ShowPreviews      bool       `json:"show_previews"`
	IncludeAIHandled  bool       `json:"include_ai_handled"`
}

/*
NewNotificationSettings shapes settings for the API.
*/
func NewNotificationSettings(s models.NotificationSetting) NotificationSettings {
	bypass := s.QuietHoursBypass
	if bypass == nil {
		bypass = []string{}
	}
	return NotificationSettings{
		Timezone: s.Timezone, QuietHoursEnabled: s.QuietHoursEnabled, QuietHoursStart: s.QuietHoursStart,
		QuietHoursEnd: s.QuietHoursEnd, QuietHoursDays: s.QuietHoursDays, QuietHoursBypass: bypass,
		PausedUntil: s.PausedUntil, ShowPreviews: s.ShowPreviews, IncludeAIHandled: s.IncludeAiHandled,
	}
}

/*
NewNotificationPreferences shapes the matrix for the API.
*/
func NewNotificationPreferences(m notifications.Matrix) NotificationPreferences {
	out := NotificationPreferences{
		Channels: notifications.Channels(), Settings: NewNotificationSettings(m.Settings),
		Triggers: make([]NotificationTrigger, 0, len(m.Triggers)), Cells: make([]NotificationCell, 0, len(m.Cells)),
	}
	for _, t := range m.Triggers {
		out.Triggers = append(out.Triggers, NotificationTrigger{Key: t.Key, Label: t.Label, Description: t.Description})
	}
	for _, c := range m.Cells {
		out.Cells = append(out.Cells, NotificationCell(c))
	}
	return out
}
