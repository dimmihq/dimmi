/*
|--------------------------------------------------------------------------
| Snooze Requests
|--------------------------------------------------------------------------
|
| A snooze list is set whole for the workspace or a team; sending null
| removes it so it inherits again. The options are checked by the
| action, which knows every kind of option.
|
*/

package requests

/*
SnoozeOptionBody is one snooze choice as a request sends it.
*/
type SnoozeOptionBody struct {
	Key       string `json:"key"                  validate:"required,max=40"`
	Label     string `json:"label"                validate:"required,max=60"`
	Kind      string `json:"kind"                 validate:"required,oneof=preset in day_at weekday_at"`
	Preset    string `json:"preset,omitempty"`
	Minutes   int    `json:"minutes,omitempty"`
	DaysAhead int    `json:"days_ahead,omitempty"`
	Weekday   int    `json:"weekday,omitempty"`
	At        string `json:"at,omitempty"`
}

/*
UpdateSnoozeListRequest is the input of PATCH /snooze-lists/{scope}, the
scope being workspace or a team's id. Options left out or null remove
the list.
*/
type UpdateSnoozeListRequest struct {
	Scope string `path:"scope"`
	Body  struct {
		Options []SnoozeOptionBody `json:"options" maxItems:"10"`
	}
}
