/*
|--------------------------------------------------------------------------
| Slack Room Controller
|--------------------------------------------------------------------------
|
| GET /channels/{channel}/rooms lists the Slack channels a connection
| watches, removed ones included so the page can ask for the app to be
| invited back. PATCH /channels/{channel}/rooms/{room} changes one room's
| mode, trigger and settings. An id of another workspace's connection or
| room is simply not found.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SlackRoomController serves the rooms of Slack connections.
*/
type SlackRoomController struct {
	Slack *channels.Slack
	IDs   *ids.Registry
}

/*
SlackRoomsOutput is a connection's rooms.
*/
type SlackRoomsOutput struct {
	Body fhttp.Collection[resources.SlackRoom]
}

/*
SlackRoomOutput is one room.
*/
type SlackRoomOutput struct {
	Body fhttp.Resource[resources.SlackRoom]
}

/*
Index lists the connection's rooms.
*/
func (c *SlackRoomController) Index(ctx context.Context, in *requests.SlackRoomsRequest) (*SlackRoomsOutput, error) {
	connection, err := c.IDs.Decode(string(models.PrefixChannelConnection), in.Channel)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	rooms, err := c.Slack.Rooms(ctx, membership(ctx), connection)
	if err != nil {
		return nil, err
	}

	items := resources.NewSlackRooms(rooms)

	return &SlackRoomsOutput{Body: fhttp.NewCollection(pagination.Page[resources.SlackRoom]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
Update changes one room.
*/
func (c *SlackRoomController) Update(ctx context.Context, in *requests.UpdateSlackRoomRequest) (
	*SlackRoomOutput, error,
) {
	connection, err := c.IDs.Decode(string(models.PrefixChannelConnection), in.Channel)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	room, err := c.IDs.Decode(string(models.PrefixChannelRoom), in.Room)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	changed, err := c.Slack.ChangeRoom(ctx, channels.ChangeRoomInput{
		Membership: membership(ctx), ConnectionID: connection, RoomID: room, Mode: in.Body.Mode,
		Trigger: in.Body.Trigger, Set: roomSettings(in.Body.Settings), Unset: in.ClearedSettings(),
	})
	if err != nil {
		return nil, err
	}

	return &SlackRoomOutput{Body: fhttp.NewResource(resources.NewSlackRoom(changed))}, nil
}

/*
roomSettings are the settings the change sends a value for.
*/
func roomSettings(body *requests.SlackRoomSettingsBody) map[string]any {
	set := map[string]any{}
	if body == nil {
		return set
	}

	put(set, "agent_mode", body.AgentMode)
	put(set, "agent_delay_seconds", body.AgentDelaySeconds)
	put(set, "agent_delay_when", body.AgentDelayWhen)
	put(set, "triage_channel_id", body.TriageChannelID)
	put(set, "auto_reply_after_seconds", body.AutoReplyAfterSeconds)
	put(set, "auto_join", body.AutoJoin)
	put(set, "request_sensitivity", body.RequestSensitivity)

	return set
}

/*
put writes the value under the key when one was sent.
*/
func put[T any](set map[string]any, key string, value *T) {
	if value != nil {
		set[key] = *value
	}
}
