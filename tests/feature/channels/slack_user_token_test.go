package channels_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
installedConversation connects the shared app through an installation
whose teammates reply as themselves, and returns a customer's
conversation on it.
*/
func installedConversation(t *testing.T) (*slackCase, models.Conversation) {
	t.Helper()

	sc := newSlackCase(t)
	sc.App.Actions.Slack.Channel.Secrets.Installations = grants{}
	ws := sc.Workspace.Workspace.ID
	install := query(sc, func(ctx context.Context, q *models.Queries) (models.IntegrationInstallation, error) {
		return q.CreateIntegrationInstallation(ctx, models.CreateIntegrationInstallationParams{
			ID: ids.New(), WorkspaceID: ws, Provider: "slack", ExternalAccountID: new(installTeam),
			ExternalAppID: new(appID), Name: "Acme", Source: "oauth", Scopes: []string{},
			CredentialsEncrypted: []byte{1}, Metadata: json.RawMessage(`{"bot_user_id": "U0BOT"}`),
			Now: sc.Clock.Now(),
		})
	})
	sc.dispatch(appevents.IntegrationConnected{
		Base:           appevents.Base{ID: ids.New(), WorkspaceID: ws, OccurredAt: sc.Clock.Now()},
		InstallationID: install.ID, Provider: "slack", ExternalAccountID: installTeam, ExternalAppID: appID,
		Features: []string{"channels"},
	})
	sc.Drain()
	sc.conn = query(sc, func(ctx context.Context, q *models.Queries) (models.ChannelConnection, error) {
		return q.FindInstallationChannelConnection(ctx, models.FindInstallationChannelConnectionParams{
			WorkspaceID: ws, IntegrationInstallationID: &install.ID,
		})
	})
	*sc.App.Actions.Slack.Channel.Shared = channels.SharedApp{AppID: appID, SigningSecret: "shared"}
	sc.signed("/webhooks/slack/events", "shared", "application/json",
		envelope("Ev1", message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))).AssertOK()
	sc.Drain()

	convs := sc.conversations()
	require.Len(t, convs, 1)

	return sc, convs[0]
}

func TestSlackTeammateToken(t *testing.T) {
	t.Parallel()

	t.Run("slack's trouble with a teammate's post is retried as them, never sent by the app", func(t *testing.T) {
		t.Parallel()
		sc, conv := installedConversation(t)
		sc.slack.answer("chat.postMessage", slackAnswer{status: 200, body: `{"ok": false, "error": "internal_error"}`})

		sc.replyAsOwner(conv, "On it")

		assert.Equal(t, string(enums.DeliveryStatusPending), *sc.lastReply().DeliveryStatus)
		for _, post := range sc.slack.called("chat.postMessage") {
			assert.Equal(t, "xoxp-fixture", post.Token, "no copy goes out from the app")
		}
	})
}
