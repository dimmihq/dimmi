/*
|--------------------------------------------------------------------------
| Steps And Paths
|--------------------------------------------------------------------------
|
| A step is an id, a kind and its settings; a path joins one step,
| by one of its ways out, to the next. Message steps carry what they
| send and waits how long they hold the traveler.
|
*/

package series

import "encoding/json"

/*
Node is one step: its stable ID, its Kind, its Settings in the kind's
own shape, and where the builder draws it.
*/
type Node struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Settings json.RawMessage `json:"settings,omitempty"`
	Position *Position       `json:"position,omitempty"`
}

/*
Position is where a step sits on the canvas.
*/
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

/*
Edge leads from one step, by one of its paths, to the next step.
*/
type Edge struct {
	From string `json:"from"`
	Path string `json:"path"`
	To   string `json:"to"`
}

/*
MessageSettings is a message step: Subject and Preview for email, Title
for a post, Body in Markdown, whether links are tracked, and for an
account series Recipients, the rules that pick who in the company gets
it.
*/
type MessageSettings struct {
	Subject     string          `json:"subject,omitempty"`
	Preview     string          `json:"preview,omitempty"`
	Body        string          `json:"body"`
	TrackClicks *bool           `json:"track_clicks,omitempty"`
	Recipients  json.RawMessage `json:"recipients,omitempty"`
}

/*
WaitSettings is a wait: Mode "duration" waits Amount Units, "time" until
the next At (HH:MM) on one of Weekdays (0 Sunday, none meaning any day)
in the person's zone, and "attribute" until the date in Attribute moved
by OffsetDays.
*/
type WaitSettings struct {
	Mode       string `json:"mode"`
	Amount     int    `json:"amount,omitempty"`
	Unit       string `json:"unit,omitempty"`
	At         string `json:"at,omitempty"`
	Weekdays   []int  `json:"weekdays,omitempty"`
	Attribute  string `json:"attribute,omitempty"`
	OffsetDays int    `json:"offset_days,omitempty"`
}
