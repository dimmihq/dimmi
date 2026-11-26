/*
|--------------------------------------------------------------------------
| Office Hours
|--------------------------------------------------------------------------
|
| The parts of a schedule: the stretches of each weekday, the days off,
| and the days with other hours than the week's.
|
*/

package migrations

/*
Hours is one stretch of a weekday, its times as HH:MM.
*/
type Hours struct {
	Day   string `json:"day"`
	Start string `json:"start"`
	End   string `json:"end"`
}

/*
Holiday is a day off, YYYY-MM-DD.
*/
type Holiday struct {
	Date   string `json:"date"`
	Name   string `json:"name,omitempty"`
	Yearly bool   `json:"yearly,omitempty"`
}

/*
Exception is one day with other hours than the week's: closed all day,
or open only between start and end, as HH:MM.
*/
type Exception struct {
	Date   string `json:"date"`
	Name   string `json:"name,omitempty"`
	Closed bool   `json:"closed,omitempty"`
	Start  string `json:"start,omitempty"`
	End    string `json:"end,omitempty"`
	Yearly bool   `json:"yearly,omitempty"`
}
