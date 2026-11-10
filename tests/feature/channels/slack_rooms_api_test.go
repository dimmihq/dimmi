package channels_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestSlackRoomsAPI(t *testing.T) {
	t.Parallel()

	t.Run("lists the rooms of a connection by name with their mode and trigger", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(communityRoom, "community", "ai")

		res := sc.ActingAsOwner().GetJSON("/api/v1/channels/" + sc.publicID() + "/rooms").AssertOK()

		res.AssertJSONPath("data.#", float64(2))
		res.AssertJSONPath("data.0.name", "#acme-support")
		res.AssertJSONPath("data.0.mode", "shared")
		res.AssertJSONPath("data.0.external_id", supportRoom)
		res.AssertJSONPath("data.0.status", "active")
		res.AssertJSONPath("data.0.object", "channel_room")
		res.AssertJSONPath("data.1.name", "#community")
		res.AssertJSONPath("data.1.trigger", "ai")
	})

	t.Run("changes a room's mode, trigger and settings and keeps settings it was not sent", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		room := sc.roomID(supportRoom)
		path := "/api/v1/channels/" + sc.publicID() + "/rooms/" + room
		owner := sc.ActingAsOwner()

		owner.PatchJSON(path, map[string]any{
			"mode": "helpdesk", "trigger": "ai",
			"settings": map[string]any{
				"agent_mode": "draft", "agent_delay_seconds": 120, "agent_delay_when": "inside_hours",
				"triage_channel_id": communityRoom, "auto_reply_after_seconds": 300, "auto_join": true,
				"request_sensitivity": "aggressive",
			},
		}).AssertOK().AssertJSONPath("data.mode", "helpdesk").AssertJSONPath("data.settings.agent_mode", "draft")

		owner.PatchJSON(path, map[string]any{
			"settings": map[string]any{"agent_mode": "auto", "triage_channel_id": nil},
		}).AssertOK().AssertJSONPath("data.trigger", "ai")

		stored := sc.room(supportRoom)
		assert.Equal(t, "helpdesk", stored.Mode)

		var settings map[string]any
		require.NoError(t, json.Unmarshal(stored.Settings, &settings))
		assert.Equal(t, map[string]any{
			"agent_mode": "auto", "agent_delay_seconds": float64(120), "agent_delay_when": "inside_hours",
			"auto_reply_after_seconds": float64(300), "auto_join": true, "request_sensitivity": "aggressive",
		}, settings)
	})

	t.Run("keeps an internal room's trigger off", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		path := "/api/v1/channels/" + sc.publicID() + "/rooms/" + sc.roomID(communityRoom)
		owner := sc.ActingAsOwner()

		owner.PatchJSON(path, map[string]any{"mode": "internal"}).AssertOK().AssertJSONPath("data.trigger", "off")
		owner.PatchJSON(path, map[string]any{"trigger": "every_message"}).AssertOK().
			AssertJSONPath("data.trigger", "off")
	})

	t.Run("refuses settings outside their bounds", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		path := "/api/v1/channels/" + sc.publicID() + "/rooms/" + sc.roomID(supportRoom)

		sc.ActingAsOwner().PatchJSON(path, map[string]any{
			"mode": "everyone", "trigger": "sometimes",
			"settings": map[string]any{
				"agent_mode": "maybe", "agent_delay_seconds": 901, "agent_delay_when": "never",
				"auto_reply_after_seconds": -1, "request_sensitivity": "wild",
			},
		}).AssertValidationError("mode", "trigger", "settings.agent_mode", "settings.agent_delay_seconds",
			"settings.agent_delay_when", "settings.auto_reply_after_seconds", "settings.request_sensitivity")
	})

	t.Run("lists removed rooms with their status", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.deliver(map[string]any{"type": "channel_left", "channel": supportRoom})

		res := sc.ActingAsOwner().GetJSON("/api/v1/channels/" + sc.publicID() + "/rooms").AssertOK()

		res.AssertJSONPath("data.0.status", "removed")
	})

	t.Run("refuses teammates who do not manage channels and rooms of other connections", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		path := "/api/v1/channels/" + sc.publicID() + "/rooms/" + sc.roomID(supportRoom)

		user := sc.CreateUser()
		_, err := factories.Member(t.Context(), sc.App, factories.Membership{
			Workspace: sc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)

		sc.In(user).PatchJSON(path, map[string]any{"mode": "internal"}).AssertForbidden()

		owner := sc.ActingAsOwner()
		owner.PatchJSON("/api/v1/channels/chn_nope/rooms/chr_nope", map[string]any{}).AssertNotFound()
		owner.GetJSON("/api/v1/channels/" + models.PublicID(models.PrefixChannelConnection, sc.Workspace.Workspace.ID) +
			"/rooms").AssertNotFound()
		other := models.PublicID(models.PrefixChannelRoom, sc.Workspace.Workspace.ID)
		owner.PatchJSON("/api/v1/channels/"+sc.publicID()+"/rooms/"+other, map[string]any{}).AssertNotFound()
	})
}

/*
roomID returns the public id of the connection's room of the Slack
channel.
*/
func (sc *slackCase) roomID(channel string) string {
	sc.T.Helper()

	return models.PublicID(models.PrefixChannelRoom, sc.room(channel).ID)
}
