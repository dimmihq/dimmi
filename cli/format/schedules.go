package format

/*
Schedule is office hours in a time zone with the days off. Exceptions
are days with other hours, which Holidays can't hold.
*/
type Schedule struct {
	Base
	Name       string      `json:"name"`
	Timezone   string      `json:"timezone"`
	Weekly     []Hours     `json:"weekly"`
	Holidays   []Holiday   `json:"holidays,omitempty"`
	Exceptions []Exception `json:"exceptions,omitempty"`
	Default    bool        `json:"default,omitempty"`
}

/*
Hours is one stretch of a weekday, its times as HH:MM.
*/
type Hours struct {
	Day   string `json:"day"`
	Start string `json:"start"`
	End   string `json:"end"`
}

/*
Holiday is a day off, YYYY-MM-DD; Yearly repeats it every year.
*/
type Holiday struct {
	Date   string `json:"date"`
	Name   string `json:"name,omitempty"`
	Yearly bool   `json:"yearly,omitempty"`
}

/*
Exception is a day that differs from the week: closed, or open from
Start to End, HH:MM.
*/
type Exception struct {
	Date   string `json:"date"`
	Name   string `json:"name,omitempty"`
	Closed bool   `json:"closed,omitempty"`
	Start  string `json:"start,omitempty"`
	End    string `json:"end,omitempty"`
	Yearly bool   `json:"yearly,omitempty"`
}
