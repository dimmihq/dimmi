package realtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
typingPath is the typing endpoint of the conversation.
*/
func typingPath(conversation ids.UUID) string {
	return "/api/v1/conversations/" + public(models.PrefixConversation, conversation) + "/typing-indicators"
}

/*
owner returns a client signed in as the test case's owner, working in
its workspace.
*/
func owner(tc *tests.TestCase) *testkit.Client {
	return tc.ActingAsOwner().WithHeader("Dimmi-Workspace", workspaceID(tc))
}

/*
widget connects as the contact's messenger widget and listens on the
contact's channel.
*/
func widget(t *testing.T, tc *tests.TestCase, s seed) *testkit.Realtime {
	t.Helper()

	c := testkit.ConnectRealtime(t, widgetNode(t, tc, s.contact), testkit.RealtimeOptions{Token: "visitor"})
	require.Nil(t, c.Subscribe(appbroadcasting.WidgetContactChannel(s.contact.ID)).Error)

	return c
}

/*
mark broadcasts a marker on the contact's channel, so a test can tell
that nothing was relayed before it.
*/
func mark(t *testing.T, tc *tests.TestCase, s seed) {
	t.Helper()

	require.NoError(t, tc.App.Broadcast.Broadcast(t.Context(), appbroadcasting.WidgetContactChannel(s.contact.ID),
		"marker", nil))
}

func TestTypingIndicators(t *testing.T) {
	t.Parallel()

	t.Run("relays a teammate typing a reply to the customer's widget", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		w := widget(t, tc, s)

		owner(tc).PostJSON(typingPath(s.conversation.ID), map[string]any{}).
			AssertStatus(202).
			AssertJSONPath("data.object", "typing_indicator").
			AssertJSONPath("data.state", "start").
			AssertJSONPath("data.expires_at", "2026-11-02T09:00:06Z")

		p := w.Publication()
		assert.Equal(t, "typing.started", p.Event)
		assert.JSONEq(t, `{"conversation_id":"`+public(models.PrefixConversation, s.conversation.ID)+
			`","name":"`+tc.Owner.Name+`","avatar_url":null}`, string(p.Data))
		assert.NotContains(t, string(p.Data), tc.Owner.Email)
	})

	t.Run("thins a burst of reports to one and always relays a stop", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		w := widget(t, tc, s)
		client := owner(tc)

		for range 10 {
			client.PostJSON(typingPath(s.conversation.ID), map[string]any{"state": "start"}).AssertStatus(202)
		}

		client.PostJSON(typingPath(s.conversation.ID), map[string]any{"state": "stop"}).AssertStatus(202)
		client.PostJSON(typingPath(s.conversation.ID), map[string]any{"state": "start"}).AssertStatus(202)

		assert.Equal(t, "typing.started", w.Publication().Event)
		assert.Equal(t, "typing.stopped", w.Publication().Event)
		assert.Equal(t, "typing.started", w.Publication().Event, "a stop opens the window again")
	})

	t.Run("never relays typing a note", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		w := widget(t, tc, s)

		owner(tc).PostJSON(typingPath(s.conversation.ID), map[string]any{"composer": "note"}).AssertStatus(202)
		mark(t, tc, s)

		assert.Equal(t, "marker", w.Publication().Event)
	})

	t.Run("tells a channel that cannot show typing nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "email")
		w := widget(t, tc, s)

		owner(tc).PostJSON(typingPath(s.conversation.ID), map[string]any{}).AssertStatus(202)
		mark(t, tc, s)

		assert.Equal(t, "marker", w.Publication().Event)
	})

	t.Run("does not find a conversation the caller cannot see", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, elsewhere := otherWorkspace(t, tc)
		foreign := startConversation(t, tc, elsewhere, "widget")
		trashed := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		exec(t, tc, "UPDATE conversations SET deleted_at = now() WHERE id = $1",
			trashed.conversation.ID)
		user, _ := joinAsMember(t, tc)
		member := tc.ActingAs(user).WithHeader("Dimmi-Workspace", workspaceID(tc))

		owner(tc).PostJSON(typingPath(foreign.conversation.ID), map[string]any{}).AssertNotFound()
		owner(tc).PostJSON("/api/v1/conversations/nonsense/typing-indicators", map[string]any{}).AssertNotFound()
		member.PostJSON(typingPath(trashed.conversation.ID), map[string]any{}).AssertNotFound()
	})

	t.Run("refuses a token that cannot write conversations and a bad state", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")

		tc.ActingAsToken(tc.Workspace.Owner, "conversations:read").
			PostJSON(typingPath(s.conversation.ID), map[string]any{}).AssertForbidden()
		owner(tc).PostJSON(typingPath(s.conversation.ID), map[string]any{"state": "dancing"}).
			AssertValidationError("state")
	})
}
