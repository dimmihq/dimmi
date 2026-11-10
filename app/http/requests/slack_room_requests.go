/*
|--------------------------------------------------------------------------
| Slack Room Requests
|--------------------------------------------------------------------------
|
| Listing a Slack connection's rooms and changing one. A change names
| only what it changes; a setting sent as null goes back to its default.
| Every setting is bounded here, so a room never holds a value the
| pipeline cannot read.
|
*/

package requests

import "encoding/json"

/*
SlackRoomsRequest is the input of GET /channels/{channel}/rooms.
*/
type SlackRoomsRequest struct {
	Channel string `path:"channel"`
}

/*
SlackRoomSettingsBody are the room settings a change may send: whether
the agent answers or drafts, how long it waits and when, the channel
new conversations are posted to for triage, how long before an
automatic reply, whether matching channels are joined on their own,
and how eagerly messages count as requests.
*/
type SlackRoomSettingsBody struct {
	AgentMode             *string `enum:"auto,draft"                json:"agent_mode,omitempty"`
	AgentDelaySeconds     *int    `json:"agent_delay_seconds,omitempty" maximum:"900" minimum:"0"`
	AgentDelayWhen        *string `enum:"always,inside_hours"       json:"agent_delay_when,omitempty"`
	TriageChannelID       *string `json:"triage_channel_id,omitempty" maxLength:"40" minLength:"1"`
	AutoReplyAfterSeconds *int    `json:"auto_reply_after_seconds,omitempty" maximum:"900" minimum:"0"`
	AutoJoin              *bool   `json:"auto_join,omitempty"`
	RequestSensitivity    *string `enum:"conservative,aggressive" json:"request_sensitivity,omitempty"`
}

/*
SlackRoomChangeBody is a change to a room: who counts as the customer
there, when a message becomes a conversation, and its settings.
*/
type SlackRoomChangeBody struct {
	Mode    *string `enum:"shared,community,internal,helpdesk"                    json:"mode,omitempty"`
	Trigger *string `enum:"author_window,every_message,emoji,mention,ai,auto,off" json:"trigger,omitempty"`

	Settings *SlackRoomSettingsBody `json:"settings,omitempty"`
}

/*
UpdateSlackRoomRequest is the input of PATCH
/channels/{channel}/rooms/{room}.
*/
type UpdateSlackRoomRequest struct {
	Channel string `path:"channel"`
	Room    string `path:"room"`
	Body    SlackRoomChangeBody
	RawBody []byte
}

/*
roomSettingKeys are the settings a change may set or clear.
*/
func roomSettingKeys() []string {
	return []string{
		"agent_mode", "agent_delay_seconds", "agent_delay_when", "triage_channel_id", "auto_reply_after_seconds",
		"auto_join", "request_sensitivity",
	}
}

/*
ClearedSettings returns the known settings the raw body sends as null.
*/
func (r *UpdateSlackRoomRequest) ClearedSettings() []string {
	var body struct {
		Settings json.RawMessage `json:"settings"`
	}
	_ = json.Unmarshal(r.RawBody, &body)

	nulls := Nulls(body.Settings)

	var cleared []string

	for _, key := range roomSettingKeys() {
		if nulls[key] {
			cleared = append(cleared, key)
		}
	}

	return cleared
}
