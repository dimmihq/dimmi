package realtime_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
presencePoll is how often a test reads presence while it waits for it
to settle.
*/
const presencePoll = 20 * time.Millisecond

func TestRealtimeDelivery(t *testing.T) {
	t.Parallel()

	t.Run("reaches a member connected to another node", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		other := secondNode(t, tc, tc.App.ResolveRealtime)
		c := tc.ListenAs(tc.Owner, other)
		require.Nil(t, c.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)

		dispatch(t, tc, appevents.ConversationClosed{
			Base: base(tc, ""), ConversationRef: appevents.ConversationRef{ConversationID: s.conversation.ID},
		})

		assert.Equal(t, "conversation.closed", c.Publication().Event)
	})

	t.Run("replays what a reconnecting screen missed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		url := tc.RealtimeURL()
		channel := appbroadcasting.ConversationChannel(s.conversation.ID)
		first := tc.ListenAs(tc.Owner, url)
		seen := first.Subscribe(channel)
		require.True(t, seen.Recoverable)
		first.Close()

		ref := appevents.ConversationRef{ConversationID: s.conversation.ID}
		dispatch(t, tc, appevents.ConversationClosed{Base: base(tc, ""), ConversationRef: ref})
		dispatch(t, tc, appevents.ConversationReopened{Base: base(tc, ""), ConversationRef: ref, Reason: "manual"})

		again := tc.ListenAs(tc.Owner, url)
		recovered := again.Recover(channel, seen.Epoch, seen.Offset)
		require.True(t, recovered.Recovered)
		require.Len(t, recovered.Publications, 2)
		assert.Equal(t, "conversation.closed", recovered.Publications[0].Event)
		assert.Equal(t, "conversation.reopened", recovered.Publications[1].Event)
	})

	t.Run("says recovery failed once Valkey lost the history", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		url := tc.RealtimeURL()
		channel := appbroadcasting.ConversationChannel(s.conversation.ID)
		ref := appevents.ConversationRef{ConversationID: s.conversation.ID}
		dispatch(t, tc, appevents.ConversationClosed{Base: base(tc, ""), ConversationRef: ref})
		seen := tc.ListenAs(tc.Owner, url).Subscribe(channel)

		flushRealtime(t, tc)
		dispatch(t, tc, appevents.ConversationReopened{Base: base(tc, ""), ConversationRef: ref})

		recovered := tc.ListenAs(tc.Owner, url).Recover(channel, seen.Epoch, seen.Offset)
		assert.False(t, recovered.Recovered, "the client refetches instead")
	})

	t.Run("lists a member viewing from two tabs under one user", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		url := tc.RealtimeURL()
		channel := appbroadcasting.ConversationChannel(s.conversation.ID)
		first, second := tc.ListenAs(tc.Owner, url), tc.ListenAs(tc.Owner, url)
		require.Nil(t, first.Subscribe(channel).Error)
		require.Nil(t, second.Subscribe(channel).Error)

		listeners, refusal := first.Presence(channel)
		require.Nil(t, refusal)
		require.Len(t, listeners, 2)

		user := public(models.PrefixUser, tc.Owner.ID)
		for _, l := range listeners {
			assert.Equal(t, user, l.User)

			var info appbroadcasting.Info
			require.NoError(t, json.Unmarshal(l.Info, &info))
			assert.Equal(t, tc.Owner.Name, info.Name)
		}

		second.Close()
		require.Eventually(t, func() bool {
			left, err := tc.App.Broadcast.Presence(t.Context(), channel)

			return err == nil && len(left) == 1 && left[0].UserID == user
		}, testkit.RealtimeWait, presencePoll)
	})

	t.Run("never replays someone typing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		url := tc.RealtimeURL()
		channel := appbroadcasting.ConversationChannel(s.conversation.ID)
		live := tc.ListenAs(tc.Owner, url)
		seen := live.Subscribe(channel)
		ref := appevents.ConversationRef{ConversationID: s.conversation.ID}

		for _, started := range []bool{true, false} {
			dispatch(t, tc, appevents.ContactTyping{
				Base: base(tc, ""), ConversationRef: ref, ContactID: s.contact.ID, Started: started,
			})
		}

		assert.Equal(t, "typing.started", live.Publication().Event)
		stopped := live.Publication()
		assert.Equal(t, "typing.stopped", stopped.Event)
		assert.Contains(t, string(stopped.Data), public(models.PrefixContact, s.contact.ID))

		late := tc.ListenAs(tc.Owner, url).Recover(channel, seen.Epoch, seen.Offset)
		assert.Empty(t, late.Publications)
	})
}

func TestWorkspaceBroadcasts(t *testing.T) {
	t.Parallel()

	t.Run("tells the workspace about a rename and member changes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := joinAsMember(t, tc)
		c := tc.ListenAs(user, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)
		admin := owner(tc)

		admin.PatchJSON("/api/v1/workspace", map[string]any{"name": "Renamed"}).AssertOK()
		renamed := c.Publication()
		assert.Equal(t, "workspace.updated", renamed.Event)
		assert.JSONEq(t, `{"id":"`+public(models.PrefixWorkspace, tc.Workspace.Workspace.ID)+
			`","object":"workspace","changes":["name"]}`, string(renamed.Data))

		admin.PatchJSON("/api/v1/members/"+public(models.PrefixMember, member.ID), map[string]any{"role": "admin"}).
			AssertOK()
		assert.Equal(t, "member.role_changed", c.Publication().Event)

		for _, e := range []any{
			appevents.MemberJoined{WorkspaceID: tc.Workspace.Workspace.ID, MemberID: member.ID},
			appevents.MemberReactivated{WorkspaceID: tc.Workspace.Workspace.ID, MemberID: member.ID},
		} {
			dispatch(t, tc, e)
			assert.True(t, strings.HasPrefix(c.Publication().Event, "member."))
		}
	})

	t.Run("closes a deactivated member's connections for good", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		user, member := joinAsMember(t, tc)
		url := tc.RealtimeURL()
		removed := tc.ListenAs(user, url)
		require.Nil(t, removed.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)
		teammate := tc.ListenAs(tc.Owner, url)
		require.Nil(t, teammate.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)

		deactivate := map[string]any{"status": "deactivated"}
		owner(tc).
			PatchJSON("/api/v1/members/"+public(models.PrefixMember, member.ID), deactivate).
			AssertOK()

		assert.Equal(t, 4009, removed.Closed())
		assert.Equal(t, "member.deactivated", teammate.Publication().Event)

		again := tc.ListenAs(user, url)
		assert.NotNil(t, again.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)
	})
}

/*
flushRealtime deletes every realtime key of the test, as a Valkey
restart would.
*/
func flushRealtime(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	rdb := tc.App.Redis.Rueidis()
	pattern := strings.TrimSuffix(tc.App.Redis.Prefix(broadcasting.DefaultKeyspace), ":") + "*"

	keys, err := rdb.Do(t.Context(), rdb.B().Keys().Pattern(pattern).Build()).AsStrSlice()
	require.NoError(t, err)
	require.NotEmpty(t, keys)
	require.NoError(t, rdb.Do(t.Context(), rdb.B().Del().Key(keys...).Build()).Error())
}
