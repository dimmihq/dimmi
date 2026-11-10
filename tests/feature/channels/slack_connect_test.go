package channels_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

func TestSlackConnect(t *testing.T) {
	t.Parallel()

	t.Run("connects an app with its secrets and reads the channels it is in", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		assert.Equal(t, "active", sc.conn.Status)
		require.NotNil(t, sc.conn.ExternalID)
		assert.Equal(t, appID+":"+installTeam, *sc.conn.ExternalID)

		facts := channels.FactsOf(sc.conn)
		assert.Equal(t, botUser, facts.BotUserID)
		assert.Equal(t, "Acme", facts.TeamName)

		shared := sc.room(supportRoom)
		assert.Equal(t, string(enums.RoomModeShared), shared.Mode)
		assert.Equal(t, "#acme-support", shared.Name)
		assert.Equal(t, string(enums.RoomModeCommunity), sc.room(communityRoom).Mode)
	})

	t.Run("refuses tokens Slack rejects, of the wrong kind or without their secret", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		sc.slack.answer("auth.test", slackAnswer{status: 200, body: `{"ok": false, "error": "invalid_auth"}`})
		owner := sc.ActingAsOwner()

		owner.PostJSON("/api/v1/channels/slack", map[string]any{"bot_token": slackBot, "signing_secret": "s"}).
			AssertJSONPath("code", "slack_token_rejected")
		owner.PostJSON("/api/v1/channels/slack", map[string]any{"bot_token": "xoxp-user", "signing_secret": "s"}).
			AssertJSONPath("code", "slack_token_rejected")
		owner.PostJSON("/api/v1/channels/slack", map[string]any{"bot_token": slackBot}).
			AssertJSONPath("code", "slack_signing_secret_required")
	})

	t.Run("refuses an app created without every scope the channel needs", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		sc.slack.answer("auth.test", slackAnswer{
			status: 200, body: `{"ok": true, "team_id": "T0ACME", "user_id": "U0BOT", "bot_id": "B0BOT"}`,
			header: map[string]string{"X-OAuth-Scopes": "chat:write,users:read"},
		})

		sc.ActingAsOwner().PostJSON("/api/v1/channels/slack", map[string]any{
			"bot_token": slackBot, "signing_secret": slackSigning,
		}).AssertJSONPath("code", "slack_scopes_missing")
	})

	t.Run("an app on a socket needs an app token Slack accepts", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)

		sc.ActingAsOwner().PostJSON("/api/v1/channels/slack", map[string]any{
			"bot_token": slackBot, "signing_secret": slackSigning, "inbound_transport": "socket",
		}).AssertJSONPath("code", "slack_socket_mode_disabled")

		res := sc.ActingAsOwner().PostJSON("/api/v1/channels/slack", map[string]any{
			"bot_token": slackBot, "signing_secret": slackSigning, "inbound_transport": "socket",
			"app_token": "xapp-test",
		}).AssertCreated()
		assert.Equal(t, "socket", res.String("data.inbound_transport"))
	})

	t.Run("the same app twice in one workspace says it is connected already", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.ActingAsOwner().PostJSON("/api/v1/channels/slack", map[string]any{
			"bot_token": slackBot, "signing_secret": slackSigning,
		}).AssertJSONPath("code", "channel_already_connected")
	})

	t.Run("secrets set later make a pending connection live", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		owner := sc.ActingAsOwner()
		id := owner.PostJSON("/api/v1/channels/slack", map[string]any{"name": "Later"}).AssertCreated().
			String("data.id")

		res := owner.PostJSON("/api/v1/channels/"+id+"/credentials", map[string]any{
			"bot_token": slackBot, "signing_secret": slackSigning,
		}).AssertOK()
		assert.Equal(t, "active", res.String("data.status"))
		assert.NotContains(t, string(res.Body), slackBot)

		owner.PostJSON("/api/v1/channels/chn_nothing/credentials", map[string]any{"bot_token": slackBot}).
			AssertNotFound()
	})

	t.Run("lands in the inbox named, and refuses one that is not the workspace's", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		inbox := query(sc, func(ctx context.Context, q *models.Queries) (models.Inbox, error) {
			return q.FindDefaultInbox(ctx, sc.Workspace.Workspace.ID)
		})
		owner := sc.ActingAsOwner()

		owner.PostJSON("/api/v1/channels/slack", map[string]any{
			"inbox_id": models.PublicID(models.PrefixInbox, inbox.ID),
		}).AssertCreated()
		owner.PostJSON("/api/v1/channels/slack", map[string]any{
			"inbox_id": models.PublicID(models.PrefixInbox, ids.New()),
		}).AssertNotFound()
		owner.PostJSON("/api/v1/channels/slack", map[string]any{"inbox_id": "inbox_nope"}).AssertNotFound()
	})
}
