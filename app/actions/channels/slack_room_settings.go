/*
|--------------------------------------------------------------------------
| Slack Room Settings
|--------------------------------------------------------------------------
|
| The settings page lists the Slack channels a connection watches and
| lets an admin decide, per channel, who counts as the customer, when a
| message becomes a conversation and how the agent answers. Changing a
| room needs channels:manage. Settings merge into what the room already
| holds, so a key the page does not know about is never lost. A room
| for the team's own talk never turns messages into conversations, so
| an internal room's trigger is always off.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ChangeRoomInput is a change to one room of a Slack connection. Mode and
Trigger are kept when nil. Set holds the settings to write and Unset
the settings to drop.
*/
type ChangeRoomInput struct {
	Membership   authz.Membership
	ConnectionID ids.UUID
	RoomID       ids.UUID
	Mode         *string
	Trigger      *string
	Set          map[string]any
	Unset        []string
}

/*
Rooms returns the rooms of the workspace's Slack connection, removed
ones included, by name.

Errors: [exceptions.ErrNotFound] for another workspace's or another
type's connection.
*/
func (s *Slack) Rooms(ctx context.Context, m authz.Membership, connectionID ids.UUID) ([]models.ChannelRoom, error) {
	ctx = tenancy.With(ctx, m.Workspace.ID.String())

	if err := s.slackConnection(ctx, m.Workspace.ID, connectionID); err != nil {
		return nil, err
	}

	var rooms []models.ChannelRoom

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		rooms, err = q.ListChannelRooms(ctx, models.ListChannelRoomsParams{
			WorkspaceID: m.Workspace.ID, ChannelConnectionID: connectionID,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list slack rooms: %w", err)
	}

	return rooms, nil
}

/*
ChangeRoom changes how one room of the workspace's Slack connection is
watched.

Errors: [exceptions.ErrForbidden] without channels:manage, and
[exceptions.ErrNotFound] for a connection or room that is not the
workspace's.
*/
func (s *Slack) ChangeRoom(ctx context.Context, in ChangeRoomInput) (models.ChannelRoom, error) {
	if !in.Membership.Can("channels:manage") {
		return models.ChannelRoom{}, exceptions.ErrForbidden
	}

	ws := in.Membership.Workspace.ID
	ctx = tenancy.With(ctx, ws.String())

	if err := s.slackConnection(ctx, ws, in.ConnectionID); err != nil {
		return models.ChannelRoom{}, err
	}

	set, err := json.Marshal(nonNilSettings(in.Set))
	if err != nil {
		return models.ChannelRoom{}, fmt.Errorf("encode room settings: %w", err)
	}

	var room models.ChannelRoom

	err = s.read(ctx, func(q *models.Queries) error {
		current, err := roomOf(ctx, q, in)
		if err != nil {
			return err
		}

		room, err = q.ChangeChannelRoomSettings(ctx, models.ChangeChannelRoomSettingsParams{
			Mode: in.Mode, Trigger: triggerFor(current, in), Set: set, Unset: nonNilList(in.Unset),
			Now: s.Clock.Now(), WorkspaceID: ws, ChannelConnectionID: in.ConnectionID, ID: in.RoomID,
		})

		return err
	})
	if err != nil {
		return room, fmt.Errorf("change slack room: %w", err)
	}

	return room, nil
}

/*
slackConnection makes sure the id names the workspace's live Slack
connection, answering ErrNotFound otherwise.
*/
func (s *Slack) slackConnection(ctx context.Context, ws, id ids.UUID) error {
	conn, err := s.connection(ctx, ws, id)
	gone := err == nil && (conn.Type != slackdriver.Type || conn.DeletedAt != nil)

	if errors.Is(err, database.ErrNotFound) || gone {
		return exceptions.ErrNotFound
	}

	return err
}

/*
roomOf returns the room the change names, or ErrNotFound when it is not
one of the connection's.
*/
func roomOf(ctx context.Context, q *models.Queries, in ChangeRoomInput) (models.ChannelRoom, error) {
	room, err := q.FindChannelRoom(ctx, models.FindChannelRoomParams{
		WorkspaceID: in.Membership.Workspace.ID, ID: in.RoomID,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && room.ChannelConnectionID != in.ConnectionID) {
		return room, exceptions.ErrNotFound
	}

	return room, err
}

/*
triggerFor is the trigger the room ends with: off whenever the room is
internal, the asked one otherwise.
*/
func triggerFor(current models.ChannelRoom, in ChangeRoomInput) *string {
	mode := current.Mode
	if in.Mode != nil {
		mode = *in.Mode
	}

	if mode == string(enums.RoomModeInternal) {
		off := string(enums.RoomTriggerOff)

		return &off
	}

	return in.Trigger
}

func nonNilSettings(set map[string]any) map[string]any {
	if set == nil {
		return map[string]any{}
	}

	return set
}

func nonNilList(list []string) []string {
	if list == nil {
		return []string{}
	}

	return list
}
