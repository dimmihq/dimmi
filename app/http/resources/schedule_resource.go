/*
|--------------------------------------------------------------------------
| Schedule Resources
|--------------------------------------------------------------------------
|
| How a schedule reads over the API: its hours as stored, and where it
| stands right now, open or not, with when it next opens and closes, so
| a client never does timezone arithmetic of its own.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
Schedule is a schedule with its status at the moment it was read.
*/
type Schedule struct {
	ID        string          `json:"id"`
	Object    string          `json:"object"`
	Name      string          `json:"name"`
	Timezone  string          `json:"timezone"`
	IsDefault bool            `json:"is_default"`
	Weekly    json.RawMessage `json:"weekly"`
	Holidays  json.RawMessage `json:"holidays"`
	Status    ScheduleStatus  `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

/*
ScheduleStatus says whether the schedule is open now, when it next
opens and, while open, when it closes.
*/
type ScheduleStatus struct {
	Open      bool       `json:"open"`
	NextOpen  *time.Time `json:"next_open"`
	NextClose *time.Time `json:"next_close"`
}

/*
NewSchedule shapes a schedule as it stands at now.
*/
func NewSchedule(s models.Schedule, now time.Time) Schedule {
	out := Schedule{
		ID: models.PublicID(models.PrefixSchedule, s.ID), Object: "schedule", Name: s.Name, Timezone: s.Timezone,
		IsDefault: s.IsDefault, Weekly: s.Weekly, Holidays: s.Holidays, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}

	parsed, err := officehours.ParseStored(s.Timezone, s.Weekly, s.Holidays)
	if err != nil {
		return out
	}

	out.Status.Open = parsed.IsOpen(now)
	if end, ok := parsed.NextClose(now); ok {
		out.Status.NextClose = &end
	}

	if next, err := parsed.NextOpen(now); err == nil && !out.Status.Open {
		out.Status.NextOpen = &next
	}

	return out
}

/*
NewSchedules shapes a list of schedules.
*/
func NewSchedules(rows []models.Schedule, now time.Time) []Schedule {
	out := make([]Schedule, len(rows))
	for i, s := range rows {
		out[i] = NewSchedule(s, now)
	}

	return out
}
