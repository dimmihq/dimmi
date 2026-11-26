/*
|--------------------------------------------------------------------------
| Saved Reply And Schedule Records
|--------------------------------------------------------------------------
|
| Canned answers with what sending them does, and office hours with
| their days off.
|
*/

package migrations

/*
SavedReply is a canned answer: a Liquid body and what sending it does.
*/
type SavedReply struct {
	Base
	Title    string        `json:"title"`
	Body     string        `json:"body"`
	Shortcut string        `json:"shortcut,omitempty"`
	Shared   *bool         `json:"shared,omitempty"`
	OwnerRef string        `json:"owner_ref,omitempty"`
	Actions  []ReplyAction `json:"actions,omitempty"`
}

/*
ReplyAction is what a saved reply does when sent: add a tag, assign, or
change the status.
*/
type ReplyAction struct {
	Type        string `json:"type"`
	TagRef      string `json:"tag_ref,omitempty"`
	AssigneeRef string `json:"assignee_ref,omitempty"`
	TeamRef     string `json:"team_ref,omitempty"`
	Status      string `json:"status,omitempty"`
}

/*
Schedule is office hours in a time zone with the days off.
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
