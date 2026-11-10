/*
|--------------------------------------------------------------------------
| Slack Room Resource
|--------------------------------------------------------------------------
|
| A watched Slack channel as the settings page shows it: its name and
| whether the app is still in it, who counts as the customer there, when
| a message becomes a conversation, and the settings an admin chose. A
| removed room keeps its settings for when the app is invited back.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackRoom is one watched Slack channel. ExternalID is Slack's id of the
channel and Shared says it is shared with another organization.
*/
type SlackRoom struct {
	ID              string         `json:"id"`
	Object          string         `json:"object"`
	ChannelID       string         `json:"channel_id"`
	ExternalID      string         `json:"external_id"`
	Name            string         `json:"name"`
	Status          string         `json:"status"`
	Mode            string         `json:"mode"`
	Trigger         string         `json:"trigger"`
	Shared          bool           `json:"is_shared_external"`
	Settings        map[string]any `json:"settings"`
	LastError       *string        `json:"last_error"`
	LastSeenEventAt *time.Time     `json:"last_seen_event_at"`
	RemovedAt       *time.Time     `json:"removed_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

/*
NewSlackRoom shapes a room for the API.
*/
func NewSlackRoom(room models.ChannelRoom) SlackRoom {
	out := SlackRoom{
		ID: models.PublicID(models.PrefixChannelRoom, room.ID), Object: "channel_room",
		ChannelID:  models.PublicID(models.PrefixChannelConnection, room.ChannelConnectionID),
		ExternalID: room.ExternalID, Name: room.Name, Status: room.Status, Mode: room.Mode, Trigger: room.Trigger,
		Shared: room.IsSharedExternal, Settings: map[string]any{}, LastError: room.LastError,
		LastSeenEventAt: room.LastSeenEventAt, RemovedAt: room.RemovedAt, CreatedAt: room.CreatedAt,
		UpdatedAt: room.UpdatedAt,
	}
	_ = json.Unmarshal(room.Settings, &out.Settings)

	if out.Settings == nil {
		out.Settings = map[string]any{}
	}

	return out
}

/*
NewSlackRooms shapes a list of rooms.
*/
func NewSlackRooms(rooms []models.ChannelRoom) []SlackRoom {
	out := make([]SlackRoom, 0, len(rooms))
	for _, room := range rooms {
		out = append(out, NewSlackRoom(room))
	}

	return out
}
