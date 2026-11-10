package channels_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/models"
)

func TestSlackRouter(t *testing.T) {
	t.Parallel()

	t.Run("hands a direct message to the narrower claim first and falls through", func(t *testing.T) {
		t.Parallel()
		r := channels.NewRouter()

		var seen []string

		r.OnEvent("message", func(context.Context, channels.SlackEvent) (bool, error) {
			seen = append(seen, "message")

			return true, nil
		})
		r.OnEvent("message.im", func(_ context.Context, ev channels.SlackEvent) (bool, error) {
			seen = append(seen, "message.im")

			return ev.Event.User == "U0LINKED", nil
		})

		dm := channels.SlackEvent{Event: slackdriver.Event{Type: "message", ChannelType: "im", User: "U0LINKED"}}
		handled, err := r.DispatchEvent(t.Context(), dm)
		require.NoError(t, err)
		assert.True(t, handled)
		assert.Equal(t, []string{"message.im"}, seen)

		dm.Event.User = "U0OTHER"
		_, err = r.DispatchEvent(t.Context(), dm)
		require.NoError(t, err)
		assert.Equal(t, []string{"message.im", "message.im", "message"}, seen)

		home := channels.SlackEvent{Event: slackdriver.Event{Type: "app_home_opened"}}
		handled, err = r.DispatchEvent(t.Context(), home)
		require.NoError(t, err)
		assert.False(t, handled)
	})

	t.Run("hands block actions to the longest prefix, shortcuts and forms by callback id", func(t *testing.T) {
		t.Parallel()
		r := channels.NewRouter()

		var seen []string

		claim := func(name string) channels.InteractionHandler {
			return func(context.Context, channels.SlackInteraction) (bool, error) {
				seen = append(seen, name)

				return true, nil
			}
		}

		r.OnAction("triage:", claim("triage"))
		r.OnAction("triage:take", claim("take"))
		r.OnShortcut("discuss", claim("discuss"))
		r.OnView("intake", claim("intake"))

		for _, in := range []slackdriver.Interaction{
			{Type: slackdriver.InteractionBlockActions, Actions: actions("triage:take:1")},
			{Type: slackdriver.InteractionBlockActions, Actions: actions("triage:snooze")},
			{Type: slackdriver.InteractionMessageAction, CallbackID: "discuss"},
			{Type: slackdriver.InteractionViewSubmit, CallbackID: "intake"},
			{Type: "unknown"},
		} {
			_, err := r.DispatchInteraction(t.Context(), channels.SlackInteraction{Interaction: in})
			require.NoError(t, err)
		}

		assert.Equal(t, []string{"take", "triage", "discuss", "intake"}, seen)
	})
}

/*
actions returns block actions with the ids.
*/
func actions(actionIDs ...string) []slackdriver.BlockAction {
	out := make([]slackdriver.BlockAction, 0, len(actionIDs))
	for _, id := range actionIDs {
		out = append(out, slackdriver.BlockAction{ActionID: id})
	}

	return out
}

func TestSlackInstallationOnlyRouting(t *testing.T) {
	t.Parallel()

	t.Run("an install without channels still hands its presses and messages to the features", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		sc.App.Actions.Slack.Channel.Secrets.Installations = grants{}
		*sc.App.Actions.Slack.Channel.Shared = channels.SharedApp{AppID: appID, SigningSecret: "shared"}
		ws := sc.Workspace.Workspace.ID

		install := query(sc, func(ctx context.Context, q *models.Queries) (models.IntegrationInstallation, error) {
			return q.CreateIntegrationInstallation(ctx, models.CreateIntegrationInstallationParams{
				ID: ids.New(), WorkspaceID: ws, Provider: "slack", ExternalAccountID: new(installTeam),
				ExternalAppID: new(appID), Name: "Acme", Source: "oauth", Scopes: []string{},
				CredentialsEncrypted: []byte{1}, Metadata: json.RawMessage(`{"bot_user_id": "U0BOT"}`),
				Now: sc.Clock.Now(),
			})
		})

		var pressed []channels.SlackInteraction

		var direct []channels.SlackEvent

		router := sc.App.Actions.Slack.Router
		router.OnAction("dimmi_notify:test:", func(_ context.Context, in channels.SlackInteraction) (bool, error) {
			pressed = append(pressed, in)

			return true, nil
		})
		router.OnEvent("message.im", func(_ context.Context, ev channels.SlackEvent) (bool, error) {
			direct = append(direct, ev)

			return true, nil
		})

		raw, err := json.Marshal(map[string]any{
			"type": "block_actions", "api_app_id": appID, "trigger_id": "trig-1",
			"team": map[string]any{"id": installTeam}, "user": map[string]any{"id": "U0TEAMMATE"},
			"actions": []map[string]any{{"action_id": "dimmi_notify:test:open", "action_ts": "1793000100.000100"}},
		})
		require.NoError(t, err)
		sc.signed("/webhooks/slack/interactions", "shared", "application/x-www-form-urlencoded",
			[]byte(url.Values{"payload": {string(raw)}}.Encode())).AssertOK()

		im := message("D0DIRECT", "U0TEAMMATE", rootTS, "hello Operator")
		im["channel_type"] = "im"
		sc.signed("/webhooks/slack/events", "shared", "application/json", envelope("Ev1", im)).AssertOK()
		sc.Drain()

		require.Len(t, pressed, 1)
		assert.False(t, pressed[0].Connected())
		assert.Equal(t, ws, pressed[0].WorkspaceID)
		require.NotNil(t, pressed[0].InstallationID)
		assert.Equal(t, install.ID, *pressed[0].InstallationID)
		assert.Equal(t, botUser, channels.FactsOf(pressed[0].Connection).BotUserID)
		require.Len(t, direct, 1)
		assert.Equal(t, ws, direct[0].WorkspaceID)

		var kept int
		require.NoError(t, sc.Operator().QueryRow(t.Context(),
			"SELECT count(*) FROM channel_inbound_events WHERE workspace_id = $1 AND channel_connection_id IS NULL "+
				"AND integration_installation_id = $2", ws, install.ID).Scan(&kept))
		assert.Equal(t, 2, kept)
	})
}
