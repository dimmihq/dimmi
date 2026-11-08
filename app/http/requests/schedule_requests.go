/*
|--------------------------------------------------------------------------
| Schedule Requests
|--------------------------------------------------------------------------
|
| The inputs of schedules: a name, a timezone, the open ranges of each
| weekday and the holidays closed all day. A save may change any of
| them, and may make the schedule the workspace's default.
|
*/

package requests

import "github.com/dimmihq/dimmi/app/support/officehours"

/*
ScheduleRequest names a schedule in the path.
*/
type ScheduleRequest struct {
	Schedule string `path:"schedule"`
}

/*
StoreScheduleRequest is the input of POST /schedules.
*/
type StoreScheduleRequest struct {
	Body struct {
		Name      string                `json:"name"               maxLength:"60"`
		Timezone  string                `json:"timezone"           maxLength:"64"`
		Weekly    officehours.Weekly    `json:"weekly"`
		Holidays  []officehours.Holiday `json:"holidays,omitempty" maxItems:"100"`
		IsDefault bool                  `json:"is_default,omitempty"`
	}
}

/*
UpdateScheduleRequest is the input of PATCH /schedules/{schedule}.
Fields left out keep what is stored.
*/
type UpdateScheduleRequest struct {
	Schedule string `path:"schedule"`
	Body     struct {
		Name      *string               `json:"name,omitempty"       maxLength:"60"`
		Timezone  *string               `json:"timezone,omitempty"   maxLength:"64"`
		Weekly    officehours.Weekly    `json:"weekly,omitempty"`
		Holidays  []officehours.Holiday `json:"holidays,omitempty"   maxItems:"100"`
		IsDefault bool                  `json:"is_default,omitempty"`
	}
}
