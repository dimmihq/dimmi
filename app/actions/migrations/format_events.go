/*
|--------------------------------------------------------------------------
| Conversation Events
|--------------------------------------------------------------------------
|
| What an event entry did: an assignment, a team, a status, a priority,
| tags, an attribute, or anything else the other tool kept.
|
*/

package migrations

import "time"

/*
Event is something that happened to a conversation: an assignment, a
team, a status, a priority, tags, an attribute, or anything else the
other tool recorded, kept with its own type and text.
*/
type Event struct {
	Type          string     `json:"type"`
	AssigneeRef   *string    `json:"assignee_ref,omitempty"`
	TeamRef       *string    `json:"team_ref,omitempty"`
	Status        string     `json:"status,omitempty"`
	Until         *time.Time `json:"until,omitempty"`
	Priority      string     `json:"priority,omitempty"`
	AddedTagRefs  []string   `json:"added_tag_refs,omitempty"`
	RemovedTagRef []string   `json:"removed_tag_refs,omitempty"`
	Attribute     string     `json:"attribute,omitempty"`
	Value         any        `json:"value,omitempty"`
	RawType       string     `json:"raw_type,omitempty"`
	Text          string     `json:"text,omitempty"`
}

/*
The kinds of event.
*/
const (
	EventAssignment     = "assignment"
	EventTeamAssignment = "team_assignment"
	EventStatus         = "status"
	EventPriority       = "priority"
	EventTag            = "tag"
	EventAttribute      = "attribute"
	EventCustom         = "custom"
)
