/*
|--------------------------------------------------------------------------
| Slack Rooms
|--------------------------------------------------------------------------
|
| A channel becomes a room the moment the app is invited to it: shared
| when it is shared with another organization, community otherwise, and
| grouping a customer's messages by default. Settings an admin chose are
| kept when the app leaves and comes back. A room follows its channel's
| life: renamed with it, moved when Slack gives it a new id, removed
| when it is archived or deleted or the app leaves, and its sharing
| recorded, never switching its mode on its own. A change in who it is
| shared with unlinks its company, so a new organization never sees the
| first one's data.
|
| An uninstalled app or a revoked bot token disconnects the connection
| and every room with it.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
onMemberJoined starts watching a channel the app was invited to.
*/
func (s *Slack) onMemberJoined(ctx context.Context, ev SlackEvent) (bool, error) {
	if ev.Event.User == "" || ev.Event.User != FactsOf(ev.Connection).BotUserID {
		return false, nil
	}

	api, err := s.api(ctx, ev.Connection)
	if err != nil {
		return false, err
	}

	channel, err := api.Channel(ctx, ev.Event.ChannelID())
	if err != nil {
		return false, err
	}

	return true, s.read(ctx, func(q *models.Queries) error { return s.upsertRoom(ctx, q, ev.Connection, channel) })
}

/*
upsertRoom watches the channel, keeping a known room's settings.
*/
func (s *Slack) upsertRoom(ctx context.Context, q *models.Queries, conn models.ChannelConnection,
	channel slackdriver.Channel,
) error {
	mode := enums.RoomModeCommunity
	if channel.IsExtShared {
		mode = enums.RoomModeShared
	}

	name := channel.Name
	if name == "" {
		name = channel.ID
	}

	_, err := q.UpsertChannelRoom(ctx, models.UpsertChannelRoomParams{
		WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, Kind: enums.RoomKindSlackChannel,
		ExternalID: channel.ID, Name: "#" + name, Mode: string(mode), Trigger: string(enums.RoomTriggerAuthorWindow),
		IsSharedExternal: channel.IsExtShared, Now: s.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("watch slack channel: %w", err)
	}

	return nil
}

/*
SyncRooms makes the connection's rooms the channels the app is in now:
new ones watched, ones it left removed.
*/
func (s *Slack) SyncRooms(ctx context.Context, workspaceID, connectionID ids.UUID) error {
	ctx = tenancy.With(ctx, workspaceID.String())

	conn, err := s.connection(ctx, workspaceID, connectionID)
	if err != nil || conn.DeletedAt != nil || conn.Status == "disconnected" {
		return ignoreMissing(err)
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return err
	}

	channels, err := api.MemberChannels(ctx, FactsOf(conn).BotUserID)
	if err != nil {
		return err
	}

	return s.read(ctx, func(q *models.Queries) error { return s.matchRooms(ctx, q, conn, channels) })
}

/*
matchRooms watches the channels and removes the rooms of any other.
*/
func (s *Slack) matchRooms(ctx context.Context, q *models.Queries, conn models.ChannelConnection,
	channels []slackdriver.Channel,
) error {
	current := make([]string, 0, len(channels))

	for _, channel := range channels {
		current = append(current, channel.ID)

		if err := s.upsertRoom(ctx, q, conn, channel); err != nil {
			return err
		}
	}

	rooms, err := q.ListChannelRooms(ctx, models.ListChannelRoomsParams{
		WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID,
	})
	if err != nil {
		return fmt.Errorf("list rooms: %w", err)
	}

	for _, room := range rooms {
		if slices.Contains(current, room.ExternalID) {
			continue
		}

		if err := s.removeRoom(ctx, q, conn, room.ExternalID); err != nil {
			return err
		}
	}

	return nil
}

/*
removeRoom stops watching the channel.
*/
func (s *Slack) removeRoom(
	ctx context.Context, q *models.Queries, conn models.ChannelConnection, channel string,
) error {
	_, err := q.RemoveChannelRoom(ctx, models.RemoveChannelRoomParams{
		WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ExternalID: channel, Now: s.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("remove room: %w", err)
	}

	return nil
}

/*
onRoomGone stops watching a channel the app left, or one archived or
deleted.
*/
func (s *Slack) onRoomGone(ctx context.Context, ev SlackEvent) (bool, error) {
	channel := ev.Event.ChannelID()
	if channel == "" {
		return false, nil
	}

	return true, s.read(ctx, func(q *models.Queries) error { return s.removeRoom(ctx, q, ev.Connection, channel) })
}

/*
onRenamed follows a channel's new name.
*/
func (s *Slack) onRenamed(ctx context.Context, ev SlackEvent) (bool, error) {
	name := ev.Event.ChannelName()
	if name == "" {
		return false, nil
	}

	return true, s.read(ctx, func(q *models.Queries) error {
		return q.RenameChannelRoom(ctx, models.RenameChannelRoomParams{
			Name: "#" + name, Now: s.Clock.Now(), WorkspaceID: ev.Connection.WorkspaceID,
			ChannelConnectionID: ev.Connection.ID, ExternalID: ev.Event.ChannelID(),
		})
	})
}

/*
onMoved follows a channel Slack gave a new id.
*/
func (s *Slack) onMoved(ctx context.Context, ev SlackEvent) (bool, error) {
	if ev.Event.OldChannelID == "" || ev.Event.NewChannelID == "" {
		return false, nil
	}

	return true, s.read(ctx, func(q *models.Queries) error {
		return q.MoveChannelRoom(ctx, models.MoveChannelRoomParams{
			NewExternalID: ev.Event.NewChannelID, Now: s.Clock.Now(), WorkspaceID: ev.Connection.WorkspaceID,
			ChannelConnectionID: ev.Connection.ID, OldExternalID: ev.Event.OldChannelID,
		})
	})
}

/*
onShared records that a channel was shared with an organization, or
stopped being shared with one.
*/
func (s *Slack) onShared(ctx context.Context, ev SlackEvent) (bool, error) {
	shared := ev.Event.Type == "channel_shared" || ev.Event.IsExtShared

	err := s.read(ctx, func(q *models.Queries) error {
		_, err := q.ShareChannelRoom(ctx, models.ShareChannelRoomParams{
			IsSharedExternal: shared, Now: s.Clock.Now(), WorkspaceID: ev.Connection.WorkspaceID,
			ChannelConnectionID: ev.Connection.ID, ExternalID: ev.Event.ChannelID(),
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}

	return err == nil, err
}

/*
onRevoked disconnects the connection when the app was uninstalled or
its bot token revoked. A revocation of teammates' own tokens leaves the
connection as it is.
*/
func (s *Slack) onRevoked(ctx context.Context, ev SlackEvent) (bool, error) {
	if ev.Connection.IntegrationInstallationID != nil && s.Revocations != nil {
		return true, s.revokeInstallation(ctx, ev)
	}

	if ev.Event.Type == "tokens_revoked" && len(ev.Event.Tokens.Bot) == 0 {
		return false, nil
	}

	reason := "Slack app was uninstalled."
	if ev.Event.Type == "tokens_revoked" {
		reason = "Slack revoked the app's token."
	}

	return true, s.disconnect(ctx, ev.Connection, reason)
}

/*
revokeInstallation reports the uninstall or revoked tokens to the
integrations, which disconnect the installation, and with it this
connection, or revoke the teammates' own grants.
*/
func (s *Slack) revokeInstallation(ctx context.Context, ev SlackEvent) error {
	facts := FactsOf(ev.Connection)
	revocation := integrations.Revocation{
		Provider: slackdriver.Type, AppID: facts.AppID, AccountID: facts.TeamID, Reason: "uninstalled", Bot: true,
	}

	if ev.Event.Type == "tokens_revoked" {
		revocation.Reason, revocation.Bot = "revoked", len(ev.Event.Tokens.Bot) > 0
		revocation.UserIDs = ev.Event.Tokens.OAuth
	}

	return s.Revocations.Handle(ctx, revocation)
}

/*
disconnect marks the connection disconnected and removes its rooms.
*/
func (s *Slack) disconnect(ctx context.Context, conn models.ChannelConnection, reason string) error {
	return s.read(ctx, func(q *models.Queries) error {
		_, err := q.MarkChannelConnection(ctx, models.MarkChannelConnectionParams{
			Status: "disconnected", LastError: &reason, Now: s.Clock.Now(), WorkspaceID: conn.WorkspaceID, ID: conn.ID,
		})
		if err != nil {
			return fmt.Errorf("disconnect slack: %w", err)
		}

		return q.RemoveConnectionRooms(ctx, models.RemoveConnectionRoomsParams{
			Now: s.Clock.Now(), WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID,
		})
	})
}

/*
settingOf returns one of a room's text settings, empty when unset.
*/
func settingOf(room *models.ChannelRoom, key string) string {
	if room == nil {
		return ""
	}

	var settings map[string]any
	if json.Unmarshal(room.Settings, &settings) != nil {
		return ""
	}

	value, _ := settings[key].(string)

	return value
}
