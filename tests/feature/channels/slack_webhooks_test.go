package channels_test

import (
	"encoding/json"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/channels"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/tests"
)

func TestSlackWebhooks(t *testing.T) {
	t.Parallel()

	t.Run("refuses unsigned, wrongly signed and stale deliveries", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		path := "/webhooks/slack/events/" + sc.publicID()
		raw := envelope("Ev1", message(supportRoom, "U0CUSTOMER", "1793000000.000100", "hi there friend"))

		sc.Client().Send("POST", path, raw).AssertUnauthorized()
		sc.signed(path, "another-secret", "application/json", raw).AssertUnauthorized()

		stale := strconv.FormatInt(sc.Clock.Now().Unix()-600, 10)
		sc.Client().WithHeader("X-Slack-Request-Timestamp", stale).
			WithHeader("X-Slack-Signature", slackdriver.SignatureHeader(slackSigning, stale, raw)).
			Send("POST", path, raw).AssertUnauthorized()

		sc.Drain()
		assert.Empty(t, sc.conversations())
	})

	t.Run("answers Slack's address check with its challenge", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		raw, _ := json.Marshal(map[string]any{"type": "url_verification", "challenge": "abc123"})

		res := sc.signed("/webhooks/slack/events/"+sc.publicID(), slackSigning, "application/json", raw).AssertOK()
		assert.Equal(t, "abc123", res.String("challenge"))
	})

	t.Run("a connection waiting for its secret only answers the address check", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		res := sc.ActingAsOwner().PostJSON("/api/v1/channels/slack", map[string]any{"name": "Pending"}).
			AssertCreated()
		assert.Equal(t, "pending", res.String("data.status"))
		assert.Contains(t, res.String("data.setup.create_app_url"), "https://api.slack.com/apps?new_app=1")
		path := "/webhooks/slack/events/" + res.String("data.id")

		check, _ := json.Marshal(map[string]any{"type": "url_verification", "challenge": "c1"})
		sc.Client().Send("POST", path, check).AssertOK().AssertJSONPath("challenge", "c1")
		sc.Client().Send("POST", path, envelope("Ev1", map[string]any{"type": "app_uninstalled"})).
			AssertUnauthorized()
	})

	t.Run("a body larger than any delivery is refused", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t, tests.WithEnv("DIMMI_CHANNELS__SLACK__MAX_PAYLOAD_BYTES=64"))
		raw := envelope("Ev1", message(supportRoom, "U0CUSTOMER", "1793000000.000100", "a long message"))

		sc.signed("/webhooks/slack/events/"+sc.publicID(), slackSigning, "application/json", raw).
			AssertStatus(413)
	})

	t.Run("the shared address leads nowhere without a shared app", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.signed("/webhooks/slack/events", slackSigning, "application/json", envelope("Ev1", nil)).
			AssertNotFound()
		sc.Client().Send("POST", "/webhooks/slack/events/chn_garbage", []byte("{}")).AssertNotFound()
	})

	t.Run("the shared app routes by app and team and drops the rest", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		*sc.App.Actions.Slack.Channel.Shared = channels.SharedApp{AppID: appID, SigningSecret: "shared-secret"}

		ev := message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails")
		sc.signed("/webhooks/slack/events", "shared-secret", "application/json", envelope("Ev1", ev)).AssertOK()

		other, _ := json.Marshal(map[string]any{
			"type": "event_callback", "team_id": "T0NOBODY", "api_app_id": "A0OTHER", "event_id": "Ev2",
			"event": message(supportRoom, "U0CUSTOMER", "1793000010.000100", "The import fails"),
		})
		sc.signed("/webhooks/slack/events", "shared-secret", "application/json", other).AssertOK()
		sc.Drain()

		require.Len(t, sc.conversations(), 1)
	})

	t.Run("the shared app routes a message shortcut, which names no app, by its team", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		*sc.App.Actions.Slack.Channel.Shared = channels.SharedApp{AppID: appID, SigningSecret: "shared-secret"}
		sc.setRoom(supportRoom, "shared", "off")

		raw, err := json.Marshal(map[string]any{
			"type": "message_action", "callback_id": slackdriver.ShortcutCreate, "trigger_id": "trig-1",
			"team": map[string]any{"id": installTeam}, "user": map[string]any{"id": "U0TEAMMATE"},
			"channel": map[string]any{"id": supportRoom},
			"message": map[string]any{"type": "message", "user": "U0CUSTOMER", "ts": "1793000000.000100", "text": "Hi"},
		})
		require.NoError(t, err)
		body := []byte(url.Values{"payload": {string(raw)}}.Encode())
		sc.signed("/webhooks/slack/interactions", "shared-secret", "application/x-www-form-urlencoded", body).AssertOK()
		sc.Drain()

		require.Len(t, sc.conversations(), 1)
	})
}
