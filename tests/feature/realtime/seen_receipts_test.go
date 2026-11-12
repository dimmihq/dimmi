package realtime_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
readPath is the read state endpoint of the conversation.
*/
func readPath(conversation ids.UUID) string {
	return "/api/v1/conversations/" + public(models.PrefixConversation, conversation) + "/read-state"
}

/*
receipt is a seen receipt as the widget gets it.
*/
type receipt struct {
	ConversationID string `json:"conversation_id"`
	SeenAt         string `json:"seen_at"`
	SeenBy         *struct {
		Name string `json:"name"`
	} `json:"seen_by"`
}

func TestSeenReceipts(t *testing.T) {
	t.Parallel()

	t.Run("tells the customer the team saw their message without naming anyone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		addMessage(t, tc, s.conversation, "Is anyone there?")
		w := widget(t, tc, s)

		owner(tc).PatchJSON(readPath(s.conversation.ID), map[string]any{}).AssertOK()

		p := w.Publication()
		require.Equal(t, "conversation.seen_by_team", p.Event)

		var got receipt
		require.NoError(t, json.Unmarshal(p.Data, &got))
		assert.Equal(t, public(models.PrefixConversation, s.conversation.ID), got.ConversationID)
		assert.NotEmpty(t, got.SeenAt)
		assert.Nil(t, got.SeenBy)
		assert.NotContains(t, string(p.Data), public(models.PrefixMember, tc.Workspace.Owner.ID))
		assert.NotContains(t, string(p.Data), tc.Owner.Name)
	})

	t.Run("names the teammate when the workspace shows names", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		addMessage(t, tc, s.conversation, "Hello?")
		w := widget(t, tc, s)

		owner(tc).PatchJSON("/api/v1/workspace", map[string]any{"seen_receipt_shows_name": true}).AssertOK().
			AssertJSONPath("data.seen_receipt_shows_name", true)
		owner(tc).PatchJSON(readPath(s.conversation.ID), map[string]any{}).AssertOK()

		var got receipt
		require.NoError(t, json.Unmarshal(w.Publication().Data, &got))
		require.NotNil(t, got.SeenBy)
		assert.Equal(t, tc.Owner.Name, got.SeenBy.Name)
	})

	t.Run("sends one receipt for a burst of reads", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		addMessage(t, tc, s.conversation, "First")
		w := widget(t, tc, s)

		owner(tc).PatchJSON(readPath(s.conversation.ID), map[string]any{}).AssertOK()
		assert.Equal(t, "conversation.seen_by_team", w.Publication().Event)

		addMessage(t, tc, s.conversation, "Second")
		owner(tc).PatchJSON(readPath(s.conversation.ID), map[string]any{}).AssertOK()
		mark(t, tc, s)

		assert.Equal(t, "marker", w.Publication().Event)
	})

	t.Run("stays quiet until the read reaches the customer's latest message", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		first := addMessage(t, tc, s.conversation, "First")
		addMessage(t, tc, s.conversation, "Second")
		w := widget(t, tc, s)

		owner(tc).PatchJSON(readPath(s.conversation.ID), map[string]any{
			"last_read_message_id": public(models.PrefixMessage, first.ID),
		}).AssertOK()
		mark(t, tc, s)

		assert.Equal(t, "marker", w.Publication().Event)
	})

	t.Run("sends nothing once the workspace turned receipts off", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		addMessage(t, tc, s.conversation, "Hello?")
		w := widget(t, tc, s)

		owner(tc).PatchJSON("/api/v1/workspace", map[string]any{"seen_receipts": false}).AssertOK().
			AssertJSONPath("data.seen_receipts", false)
		owner(tc).PatchJSON(readPath(s.conversation.ID), map[string]any{}).AssertOK()
		mark(t, tc, s)

		assert.Equal(t, "marker", w.Publication().Event)
	})

	t.Run("sends nothing for a conversation outside the messenger", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		addMessage(t, tc, s.conversation, "By API")
		w := widget(t, tc, s)

		owner(tc).PatchJSON(readPath(s.conversation.ID), map[string]any{}).AssertOK()
		mark(t, tc, s)

		assert.Equal(t, "marker", w.Publication().Event)
	})
}
