package realtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/tests"
)

/*
internal is the protocol's error code for a failure on the server.
*/
const internal = 100

func TestRealtimeFailures(t *testing.T) {
	t.Parallel()

	t.Run("refuses a subscription it cannot check, without a reason", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())

		tc.BreakTable("conversations")

		assert.Equal(t, internal, c.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error.Code)
	})

	t.Run("keeps the change when its broadcast cannot be built", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)

		tc.BreakTable("messages")
		dispatch(t, tc, appevents.MessageCreated{
			Base: base(tc, ""), MessageRef: messageRef(s.conversation.ID, s.conversation.ID),
		})
		mark := appevents.ConversationClosed{
			Base: base(tc, ""), ConversationRef: appevents.ConversationRef{ConversationID: s.conversation.ID},
		}
		dispatch(t, tc, mark)

		assert.Equal(t, "conversation.closed", c.Publication().Event, "the failed broadcast is skipped, not retried")
	})

	t.Run("reports a typing report it cannot throttle or load", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		client := owner(tc)

		tc.BreakValkey()

		client.PostJSON(typingPath(s.conversation.ID), map[string]any{}).AssertStatus(500)
		client.PostJSON(typingPath(s.conversation.ID), map[string]any{"state": "stop"}).AssertStatus(500)

		tc.BreakTable("channel_connections")
		client.PostJSON(typingPath(s.conversation.ID), map[string]any{}).AssertStatus(500)
	})
}
