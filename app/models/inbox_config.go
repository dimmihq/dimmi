/*
|--------------------------------------------------------------------------
| Inbox Settings
|--------------------------------------------------------------------------
|
| The settings an inbox keeps as JSON, read with their defaults so an
| inbox stored before a setting existed behaves as a new one: replies
| take a conversation nobody holds, closed conversations reopen within
| thirty days, and nobody is assigned automatically.
|
*/

package models

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
InboxSettings are an inbox's own settings. ReopenWindowDays is how long
after closing a reply reopens a conversation, 0 for never. AssignOnReply
gives a conversation nobody holds to whoever replies. Color marks the
inbox in lists. Assignment is how new conversations find an assignee.
*/
type InboxSettings struct {
	ReopenWindowDays int                `json:"reopen_window_days"`
	AssignOnReply    bool               `json:"assign_on_reply"`
	Color            string             `json:"color,omitempty"`
	Assignment       AssignmentSettings `json:"assignment"`
}

/*
AssignmentSettings say how an inbox assigns new conversations. Mode is
manual, round_robin or balanced. IncludeAway lets away members take
conversations. OverflowTeamID is the pool tried when the first has
nobody. QueueOrder is how waiting conversations are taken:
oldest_waiting or priority_then_oldest. DefaultCapacity is the most open
conversations of a member without their own limit, nil for none, and
CapacityByChannel the same per channel type.
*/
type AssignmentSettings struct {
	Mode              string         `json:"mode"`
	IncludeAway       bool           `json:"include_away"`
	OverflowTeamID    *ids.UUID      `json:"overflow_team_id"`
	QueueOrder        string         `json:"queue_order"`
	DefaultCapacity   *int           `json:"default_capacity"`
	CapacityByChannel map[string]int `json:"capacity_by_channel"`
}

/*
The assignment modes and queue orders.
*/
const (
	AssignmentManual             = "manual"
	AssignmentRoundRobin         = "round_robin"
	AssignmentBalanced           = "balanced"
	QueueOldestWaiting           = "oldest_waiting"
	QueuePriorityThenOldest      = "priority_then_oldest"
	DefaultInboxReopenWindowDays = 30
)

/*
DefaultInboxSettings returns the settings of a new inbox.
*/
func DefaultInboxSettings() InboxSettings {
	return InboxSettings{
		ReopenWindowDays: DefaultInboxReopenWindowDays, AssignOnReply: true,
		Assignment: AssignmentSettings{
			Mode: AssignmentManual, QueueOrder: QueueOldestWaiting, CapacityByChannel: map[string]int{},
		},
	}
}

/*
ReadInboxSettings returns stored settings with the defaults filled in
for what they lack. Settings that do not read as JSON are the defaults.
*/
func ReadInboxSettings(raw json.RawMessage) InboxSettings {
	settings := DefaultInboxSettings()
	if len(raw) == 0 {
		return settings
	}

	if err := json.Unmarshal(raw, &settings); err != nil {
		return DefaultInboxSettings()
	}

	if settings.Assignment.Mode == "" {
		settings.Assignment.Mode = AssignmentManual
	}

	if settings.Assignment.QueueOrder == "" {
		settings.Assignment.QueueOrder = QueueOldestWaiting
	}

	if settings.Assignment.CapacityByChannel == nil {
		settings.Assignment.CapacityByChannel = map[string]int{}
	}

	return settings
}

/*
ReadSettings returns the inbox's settings with their defaults.
*/
func (i Inbox) ReadSettings() InboxSettings {
	return ReadInboxSettings(i.Settings)
}
