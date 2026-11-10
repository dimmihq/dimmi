package notifications_test

import (
	"context"
	"encoding/json"
	"maps"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/oauthclient"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/actions/notifications"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

func TestSlackDestinations(t *testing.T) {
	t.Parallel()

	t.Run("posts a new conversation to the picked channel, with buttons and escaped words", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		id := s.channelDestination(itg, testkit.JSON{"triggers": []string{"new_conversation"}})

		s.Owner.GetJSON("/api/v1/notification-destinations/"+id).AssertOK().
			AssertJSONPath("data.name", "#support").AssertJSONPath("data.slack_channel_id", supportRoom).
			AssertJSONPath("data.status", "active")
		s.visitor().start("Checkout fails <!channel> & more")
		s.TC.DrainLater()

		posts := s.posts(supportRoom)
		require.Len(t, posts, 1)
		raw := posts[0].Form.Get("blocks")
		section, ok := blocks(t, posts[0])[1]["text"].(map[string]any)
		require.True(t, ok)
		quoted := section["text"]
		assert.Equal(t, ">Checkout fails &lt;!channel&gt; &amp; more", quoted)
		assert.Contains(t, raw, notifications.SlackActionTake)
		assert.Contains(t, raw, "/inbox/all/conv_")
		assert.NotNil(t, s.destination(id).LastDeliveredAt)
	})

	t.Run("hears only the triggers and inboxes it listed", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		s.channelDestination(itg, testkit.JSON{"triggers": []string{"ai_handoff_requested"}})
		s.channelDestination(itg, testkit.JSON{"triggers": []string{"new_conversation"}, "inbox_ids": []string{}})

		s.visitor().start("Hello")
		s.TC.DrainLater()

		assert.Empty(t, s.posts(supportRoom))
	})

	t.Run("refuses triggers, digests and channels it cannot use", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		base := testkit.JSON{"type": "slack_channel", "integration_id": itg, "channel_id": supportRoom}

		with := func(k string, v any) testkit.JSON {
			out := testkit.JSON{}
			maps.Copy(out, base)
			out[k] = v

			return out
		}
		s.Owner.PostJSON("/api/v1/notification-destinations", with("triggers", []string{"mentioned"})).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "invalid_trigger")
		s.Owner.PostJSON("/api/v1/notification-destinations", with("digests", []string{"hourly"})).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "invalid_digest")
		s.slack.answer("conversations.info", `{"ok": false, "error": "channel_not_found"}`)
		s.Owner.PostJSON("/api/v1/notification-destinations", base).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "slack_channel_not_found")

		member, _ := s.Join(t, enums.RoleMember)
		member.PostJSON("/api/v1/notification-destinations", base).AssertForbidden()
	})

	t.Run("a channel Slack refuses stops with the reason, and a test says so", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		id := s.channelDestination(itg, testkit.JSON{"triggers": []string{"new_conversation"}})
		s.slack.answer("chat.postMessage", `{"ok": false, "error": "is_archived"}`)

		s.Owner.PostJSON("/api/v1/notification-destinations/"+id+"/tests", testkit.JSON{}).AssertCreated().
			AssertJSONPath("data.status", "failed").AssertJSONPath("data.error_code", slackdriver.CodeChannelGone)

		row := s.destination(id)
		assert.Equal(t, notifications.StatusError, row.Status)
		s.Owner.PatchJSON("/api/v1/notification-destinations/"+id, testkit.JSON{"enabled": true}).AssertOK().
			AssertJSONPath("data.status", "active").AssertJSONPath("data.last_error_code", nil)
		s.Owner.PostJSON("/api/v1/notification-destinations/"+id+"/tests", testkit.JSON{}).AssertCreated().
			AssertJSONPath("data.status", "sent")
	})

	t.Run("an uninstalled Slack stops every destination of it", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		id := s.channelDestination(itg, testkit.JSON{"triggers": []string{"new_conversation"}})

		s.Owner.Delete("/api/v1/integrations/" + itg)
		s.TC.Drain()

		row := s.destination(id)
		assert.Equal(t, notifications.StatusError, row.Status)
		assert.Equal(t, notifications.CodeIntegrationGone, *row.LastErrorCode)
	})

	t.Run("lists the channels the bot is in and removes a destination", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		id := s.channelDestination(itg, testkit.JSON{})

		s.Owner.GetJSON("/api/v1/integrations/"+itg+"/slack-channels").AssertOK().
			AssertJSONPath("data.0.id", supportRoom).AssertJSONPath("data.0.name", "support")
		s.Owner.GetJSON("/api/v1/notification-destinations").AssertOK().AssertJSONPath("data.0.id", id)
		s.Owner.Delete("/api/v1/notification-destinations/" + id).AssertNoContent()
		s.Owner.GetJSON("/api/v1/notification-destinations/" + id).AssertNotFound()
	})
}

func TestSlackDirectMessages(t *testing.T) {
	t.Parallel()

	t.Run("a teammate found by their verified email gets their alerts as direct messages", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")

		s.Owner.PostJSON("/api/v1/notification-destinations", testkit.JSON{
			"type": "slack_dm", "integration_id": itg,
		}).AssertCreated().AssertJSONPath("data.scope", "member").AssertJSONPath("data.type", "slack_dm")
		lookups := s.slack.called("users.lookupByEmail")
		require.Len(t, lookups, 1)
		assert.Equal(t, s.TC.Owner.Email, lookups[0].Form.Get("email"))

		s.Owner.GetJSON("/api/v1/notification-preferences").AssertOK().AssertJSONPath("data.channels.3", "slack")
		s.Owner.PostJSON("/api/v1/notification-tests", testkit.JSON{"channel": "slack"}).AssertNoContent()
		s.visitor().start("My invoice is wrong")
		s.TC.DrainLater()

		posts := s.posts(ownerDM)
		require.Len(t, posts, 2)
		assert.Contains(t, posts[1].Form.Get("blocks"), "My invoice is wrong")
		assert.Len(t, s.slack.called("conversations.open"), 1)
	})

	t.Run("a teammate who connected their own Slack gets direct messages whatever their email", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		s.slack.answer("users.lookupByEmail", `{"ok": false, "error": "users_not_found"}`)

		res := s.Owner.PostJSON("/api/v1/integrations/"+itg+"/member-grants", testkit.JSON{}).AssertOK()
		callback, err := s.oauth.Approve(res.String("data.authorize_url"), oauthclient.Grant{
			UserAccess: &oauthclient.Token{Value: "xoxp-" + ids.New().String()}, UserID: ownerSlack,
			UserScopes: []string{"chat:write", "reactions:write"}, Account: slackTeam,
		})
		require.NoError(t, err)
		parsed, err := url.Parse(callback)
		require.NoError(t, err)
		s.Owner.Send(fhttp.MethodGet, parsed.RequestURI(), nil).AssertStatus(fhttp.StatusFound)

		s.Owner.PostJSON("/api/v1/notification-destinations", testkit.JSON{
			"type": "slack_dm", "integration_id": itg,
		}).AssertCreated()
		assert.Empty(t, s.slack.called("users.lookupByEmail"))
	})

	t.Run("refuses a teammate Slack does not know and one without slack yet", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		s.slack.answer("users.lookupByEmail", `{"ok": false, "error": "users_not_found"}`)

		s.Owner.PostJSON("/api/v1/notification-destinations", testkit.JSON{
			"type": "slack_dm", "integration_id": itg,
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "slack_user_not_found")
		s.Owner.PostJSON("/api/v1/notification-tests", testkit.JSON{"channel": "slack"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "channel_unreachable")
	})

	t.Run("a teammate who turned slack off for a trigger hears nothing there", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		s.Owner.PostJSON("/api/v1/notification-destinations", testkit.JSON{
			"type": "slack_dm", "integration_id": itg,
		}).AssertCreated()
		s.Owner.PatchJSON("/api/v1/notification-preferences", testkit.JSON{"preferences": []testkit.JSON{
			{"trigger": "new_conversation", "channel": "slack", "enabled": false},
		}}).AssertOK()

		s.visitor().start("Hello")
		s.TC.DrainLater()

		assert.Empty(t, s.posts(ownerDM))
	})
}

func TestSlackAlertButtons(t *testing.T) {
	t.Parallel()

	t.Run("a linked teammate takes a conversation from Slack and the alert says so", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		conn := s.connection(s.install("channels", "notifications"))
		s.link(conn, ownerSlack)
		conv := s.visitor().start("Where is my refund?")
		s.TC.Drain()

		s.press(conn, ownerSlack, notifications.SlackActionTake, conv)

		res := s.Owner.GetJSON("/api/v1/conversations/" + conv).AssertOK()
		assert.NotEmpty(t, res.String("data.assignee.id"))
		updates := s.slack.called("chat.update")
		require.Len(t, updates, 1)
		assert.Contains(t, updates[0].Form.Get("blocks"), "Taken by")
		assert.Equal(t, "1793000000.000100", updates[0].Form.Get("ts"))

		s.press(conn, ownerSlack, notifications.SlackActionUrgent, conv)
		s.Owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().AssertJSONPath("data.priority", "urgent")
	})

	t.Run("a Slack user who is not linked is asked to connect and nothing changes", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		conn := s.connection(s.install("channels", "notifications"))
		conv := s.visitor().start("Where is my refund?")
		s.TC.Drain()

		s.press(conn, "U0STRANGER", notifications.SlackActionSnooze, conv)

		ephemeral := s.slack.called("chat.postEphemeral")
		require.Len(t, ephemeral, 1)
		assert.Contains(t, ephemeral[0].Form.Get("text"), "/d/settings/integrations")
		s.Owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().AssertJSONPath("data.status", "open")
	})
}

func TestSlackDigests(t *testing.T) {
	t.Parallel()

	t.Run("posts the standup, the summary and the week's themes when their time comes", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		itg := s.install("notifications")
		id := s.channelDestination(itg, testkit.JSON{
			"digests": []string{"daily_standup", "daily_summary", "weekly_themes"}, "digest_time": "09:30",
		})
		s.visitor().start("Refund please")
		s.TC.DrainLater()
		before := len(s.posts(supportRoom))

		s.digests()
		assert.Len(t, s.posts(supportRoom), before)

		s.TC.Clock.Travel(31 * time.Minute)
		s.digests()

		posts := s.posts(supportRoom)[before:]
		require.Len(t, posts, 3)
		assert.Contains(t, posts[0].Form.Get("text"), "Daily standup")
		assert.Contains(t, posts[0].Form.Get("blocks"), "Nobody yet: 1 open")
		assert.Contains(t, posts[1].Form.Get("blocks"), "1 opened")
		assert.Contains(t, posts[1].Form.Get("blocks"), "Waiting longest without a reply")
		assert.Contains(t, posts[2].Form.Get("text"), "themes")
		next := s.destination(id).NextDigestAt
		require.NotNil(t, next)
		assert.Equal(t, 24*time.Hour, next.Sub(s.TC.Clock.Now().Add(-time.Minute)))

		s.digests()
		assert.Len(t, s.posts(supportRoom), before+3)
	})
}

/*
digests runs the digest sweep.
*/
func (s *slackDesk) digests() {
	s.t.Helper()

	require.NoError(s.t, s.TC.App.Queue.Push(s.t.Context(), jobs.SendNotificationDigestsArgs{}))
	s.TC.Drain()
}

/*
connection returns the Slack connection the installation made.
*/
func (s *slackDesk) connection(itg string) models.ChannelConnection {
	s.t.Helper()

	installation, err := s.TC.App.IDs.Decode(string(models.PrefixIntegrationInstallation), itg)
	require.NoError(s.t, err)

	var conn models.ChannelConnection

	ctx := tenancy.With(s.t.Context(), s.TC.Workspace.Workspace.ID.String())
	require.NoError(s.t, s.TC.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		params := models.FindInstallationChannelConnectionParams{
			WorkspaceID: s.TC.Workspace.Workspace.ID, IntegrationInstallationID: &installation,
		}

		var err error
		conn, err = models.New(tx).FindInstallationChannelConnection(ctx, params)

		return err
	}))

	return conn
}

/*
link links the Slack user to the owner on the connection.
*/
func (s *slackDesk) link(conn models.ChannelConnection, user string) {
	s.t.Helper()

	_, err := s.TC.Operator().Exec(s.t.Context(),
		"INSERT INTO channel_member_links (workspace_id, channel_connection_id, workspace_member_id, "+
			"external_user_id, source) VALUES ($1, $2, $3, $4, 'manual')",
		conn.WorkspaceID, conn.ID, s.TC.Workspace.Owner.ID, slackdriver.UserIdentity(slackTeam, user))
	require.NoError(s.t, err)
}

/*
press has the Slack user press the alert button about the conversation.
*/
func (s *slackDesk) press(conn models.ChannelConnection, user, action, conversation string) {
	s.t.Helper()

	raw, err := json.Marshal(map[string]any{
		"type": "block_actions", "trigger_id": ids.New().String(),
		"user": map[string]any{"id": user, "team_id": slackTeam}, "team": map[string]any{"id": slackTeam},
		"channel":   map[string]any{"id": supportRoom},
		"container": map[string]any{"type": "message", "channel_id": supportRoom, "message_ts": "1793000000.000100"},
		"actions":   []map[string]any{{"action_id": action, "value": conversation}},
	})
	require.NoError(s.t, err)

	in, err := slackdriver.ParseInteraction(raw)
	require.NoError(s.t, err)

	ctx := tenancy.With(s.t.Context(), conn.WorkspaceID.String())
	handled, err := s.TC.App.Actions.Slack.Router.DispatchInteraction(ctx, channels.SlackInteraction{
		Connection: conn, Interaction: in,
	})
	require.NoError(s.t, err)
	require.True(s.t, handled)
	s.TC.Drain()
}
