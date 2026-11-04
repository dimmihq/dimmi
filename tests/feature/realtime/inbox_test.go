package realtime_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestInboxBroadcasts(t *testing.T) {
	t.Parallel()

	t.Run("broadcasts a reply made through the api with the client's mutation id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		live := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, live.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)

		owner(tc).WithHeader("Dimmi-Client-Mutation-Id", "reply-1").
			PostJSON("/api/v1/conversations/"+public(models.PrefixConversation, s.conversation.ID)+"/replies",
				testkit.JSON{"body": "We are on it"}).
			AssertCreated()

		p := live.Publication()
		assert.Equal(t, "message.created", p.Event)
		assert.Equal(t, "reply-1", p.Meta["client_mutation_id"])
		assert.Equal(t, "member", p.Meta["actor_type"])

		var entry resources.TimelineEntry
		require.NoError(t, json.Unmarshal(p.Data, &entry))
		assert.Equal(t, "We are on it", *entry.Body)
		assert.Equal(t, tc.Owner.Name, entry.Author.Name)
	})

	t.Run("leaves the reader's unread flag out of a conversation's broadcast", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "api")
		live := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, live.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)

		owner(tc).PatchJSON("/api/v1/conversations/"+public(models.PrefixConversation, s.conversation.ID),
			testkit.JSON{"priority": true}).AssertOK()

		p := live.Publication()
		assert.Equal(t, "conversation.priority_changed", p.Event)
		assert.NotContains(t, string(p.Data), `"unread"`)
		assert.Contains(t, string(p.Data), `"priority":true`)
	})
}
