package channels_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/channels"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
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
