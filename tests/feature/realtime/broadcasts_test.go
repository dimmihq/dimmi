package realtime_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
row is the part of a conversation broadcast the tests read.
*/
type row struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Number   int64  `json:"number"`
	Assignee *struct {
		ID string `json:"id"`
	} `json:"assignee"`
}

func decodeRow(t *testing.T, p testkit.RealtimePublication) row {
	t.Helper()

	var r row
	require.NoError(t, json.Unmarshal(p.Data, &r))

	return r
}

func TestConversationBroadcasts(t *testing.T) {
	t.Parallel()

	t.Run("updates the lists and the open conversation with the client mutation id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)
		require.Nil(t, c.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)
		exec(t, tc, "UPDATE conversations SET assignee_member_id = $1 WHERE id = $2",
			tc.Workspace.Owner.ID, s.conversation.ID)

		event := appevents.ConversationAssigned{
			Base: base(tc, "m-1"), ConversationRef: appevents.ConversationRef{ConversationID: s.conversation.ID},
			ToMemberID: &tc.Workspace.Owner.ID,
		}
		dispatch(t, tc, event)

		list, open := c.Publication(), c.Publication()
		assert.Equal(t, appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID), list.Channel)
		assert.Equal(t, appbroadcasting.ConversationChannel(s.conversation.ID), open.Channel)

		for _, p := range []testkit.RealtimePublication{list, open} {
			assert.Equal(t, "conversation.assigned", p.Event)
			assert.Equal(t, ids.Encode("evt", event.ID), p.ID)
			assert.Equal(t, "m-1", p.Meta["client_mutation_id"])
			assert.Equal(t, "member", p.Meta["actor_type"])

			r := decodeRow(t, p)
			assert.Equal(t, public(models.PrefixConversation, s.conversation.ID), r.ID)
			assert.Equal(t, "conversation", r.Object)
			require.NotNil(t, r.Assignee)
			assert.Equal(t, public(models.PrefixMember, tc.Workspace.Owner.ID), r.Assignee.ID)
		}
	})

	t.Run("puts a new message on the timeline and moves the list row", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		m := addMessage(t, tc, s.conversation, "Tried twice, still declined")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)

		dispatch(t, tc, appevents.MessageCreated{
			Base:       base(tc, "m-2"),
			MessageRef: messageRef(s.conversation.ID, m.ID),
			ContactID:  s.contact.ID, Type: string(enums.MessageTypeMessage),
			AuthorType: string(enums.AuthorTypeContact), AuthorID: &s.contact.ID,
		})

		timeline := c.Publication()
		assert.Equal(t, "message.created", timeline.Event)
		assert.Equal(t, "m-2", timeline.Meta["client_mutation_id"])

		var entry resources.TimelineEntry
		require.NoError(t, json.Unmarshal(timeline.Data, &entry))
		assert.Equal(t, public(models.PrefixMessage, m.ID), entry.ID)
		assert.Equal(t, "Tried twice, still declined", *entry.Body)
		assert.Equal(t, public(models.PrefixContact, s.contact.ID), *entry.Author.ID)

		list := c.Publication()
		assert.Equal(t, appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID), list.Channel)
		assert.Equal(t, "message.created", list.Event)
		assert.Equal(t, "conversation", decodeRow(t, list).Object)

		ref := messageRef(s.conversation.ID, m.ID)
		for _, e := range []appevents.MessageEvent{
			appevents.MessageUpdated{Base: base(tc, ""), MessageRef: ref},
			appevents.MessageDeliveryUpdated{Base: base(tc, ""), MessageRef: ref, Status: "delivered"},
		} {
			dispatch(t, tc, e)

			assert.Equal(t, e.EventName(), c.Publication().Event)
			assert.Equal(t, e.EventName(), c.Publication().Event)
		}
	})

	t.Run("names member authors by id and others by type alone", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)
		owner := tc.Workspace.Owner.ID
		agent := ids.New()

		cases := []struct {
			author   enums.AuthorType
			id       *ids.UUID
			expected *string
		}{
			{author: enums.AuthorTypeMember, id: &owner, expected: new(public(models.PrefixMember, owner))},
			{author: enums.AuthorTypeAgent, id: &agent},
			{author: enums.AuthorTypeSystem},
		}
		for _, tt := range cases {
			m := addAuthoredMessage(t, tc, s.conversation, author{kind: tt.author, id: tt.id})
			e := appevents.MessageCreated{MessageRef: messageRef(s.conversation.ID, m.ID)}
			e.WorkspaceID = tc.Workspace.Workspace.ID

			dispatch(t, tc, e)

			p := c.Publication()
			assert.True(t, strings.HasPrefix(p.ID, "evt_"), "an event without an id still gets one")

			var entry resources.TimelineEntry
			require.NoError(t, json.Unmarshal(p.Data, &entry))
			assert.Equal(t, string(tt.author), entry.Author.Type)
			assert.Equal(t, tt.expected, entry.Author.ID)
		}
	})

	t.Run("hides the body of a deleted message", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		m := addMessage(t, tc, s.conversation, "secret")
		exec(t, tc, "UPDATE messages SET deleted_at = now() WHERE id = $1", m.ID)
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)

		dispatch(t, tc, appevents.MessageDeleted{Base: base(tc, ""), MessageRef: messageRef(s.conversation.ID, m.ID)})

		p := c.Publication()
		assert.Equal(t, "message.deleted", p.Event)
		assert.NotContains(t, p.Meta, "client_mutation_id")
		assert.NotContains(t, string(p.Data), "secret")
	})

	t.Run("tells only the member about their own read state and counts", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.MemberChannel(tc.Workspace.Owner.ID)).Error)
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)
		owner := appevents.MemberRef{MemberID: tc.Workspace.Owner.ID}

		dispatch(t, tc, appevents.ConversationRead{
			Base: base(tc, ""), ConversationRef: appevents.ConversationRef{ConversationID: s.conversation.ID},
			MemberRef: owner,
		})
		dispatch(t, tc, appevents.ConversationCountsChanged{Base: base(tc, ""), MemberRef: owner})

		read, counts := c.Publication(), c.Publication()
		assert.Equal(t, appbroadcasting.MemberChannel(tc.Workspace.Owner.ID), read.Channel)
		assert.Equal(t, "conversation.read", read.Event)
		assert.JSONEq(t, `{"conversation_id":"`+public(models.PrefixConversation, s.conversation.ID)+`"}`,
			string(read.Data))
		assert.Equal(t, "conversation.counts_changed", counts.Event)
		assert.Equal(t, appbroadcasting.MemberChannel(tc.Workspace.Owner.ID), counts.Channel)
	})

	t.Run("broadcasts every conversation event by its name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)

		for _, e := range conversationEvents(tc, s.conversation.ID) {
			dispatch(t, tc, e)

			p := c.Publication()
			assert.Equal(t, e.EventName(), p.Event)
			assert.Equal(t, public(models.PrefixConversation, s.conversation.ID), decodeRow(t, p).ID)
		}
	})

	t.Run("skips a conversation that no longer exists", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)
		gone := appevents.ConversationRef{ConversationID: ids.New()}

		dispatch(t, tc, appevents.ConversationClosed{Base: base(tc, ""), ConversationRef: gone})
		dispatch(t, tc, appevents.MessageCreated{
			Base: base(tc, ""), MessageRef: messageRef(gone.ConversationID, ids.New()),
		})
		dispatch(t, tc, appevents.ConversationClosed{
			Base: base(tc, ""), ConversationRef: appevents.ConversationRef{ConversationID: s.conversation.ID},
		})

		assert.Equal(t, public(models.PrefixConversation, s.conversation.ID), decodeRow(t, c.Publication()).ID)
	})

	t.Run("announces a subject too large to broadcast by reference", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		s := startConversation(t, tc, tc.Workspace.Workspace.ID, "widget")
		m := addMessage(t, tc, s.conversation, strings.Repeat("long ", broadcasting.DefaultMaxPayload/4))
		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)

		dispatch(t, tc, appevents.MessageCreated{Base: base(tc, ""), MessageRef: messageRef(s.conversation.ID, m.ID)})

		var ref appbroadcasting.Reference
		require.NoError(t, json.Unmarshal(c.Publication().Data, &ref))
		assert.Equal(t, public(models.PrefixMessage, m.ID), ref.ID)
		assert.Equal(t, "message", ref.Object)
		assert.False(t, ref.UpdatedAt.IsZero())
	})
}

func messageRef(conversation, message ids.UUID) appevents.MessageRef {
	return appevents.MessageRef{
		ConversationRef: appevents.ConversationRef{ConversationID: conversation}, MessageID: message,
	}
}

/*
conversationEvents is one of every event about a whole conversation.
*/
func conversationEvents(tc *tests.TestCase, id ids.UUID) []appevents.ConversationEvent {
	ref := appevents.ConversationRef{ConversationID: id}

	return []appevents.ConversationEvent{
		appevents.ConversationCreated{Base: base(tc, ""), ConversationRef: ref, InitiatedBy: "contact"},
		appevents.ConversationUpdated{Base: base(tc, ""), ConversationRef: ref},
		appevents.ConversationClosed{Base: base(tc, ""), ConversationRef: ref},
		appevents.ConversationReopened{Base: base(tc, ""), ConversationRef: ref, Reason: "manual"},
		appevents.ConversationSnoozed{Base: base(tc, ""), ConversationRef: ref, Until: tc.Clock.Now()},
		appevents.ConversationUnsnoozed{Base: base(tc, ""), ConversationRef: ref, Reason: "manual"},
		appevents.ConversationPriorityChanged{Base: base(tc, ""), ConversationRef: ref, Priority: true},
		appevents.ConversationTagged{Base: base(tc, ""), ConversationRef: ref},
		appevents.ConversationUntagged{Base: base(tc, ""), ConversationRef: ref},
		appevents.ConversationMarkedSpam{Base: base(tc, ""), ConversationRef: ref},
		appevents.ConversationUnmarkedSpam{Base: base(tc, ""), ConversationRef: ref},
		appevents.ConversationDeleted{Base: base(tc, ""), ConversationRef: ref},
		appevents.ConversationMerged{Base: base(tc, ""), ConversationRef: ref, SourceID: ids.New()},
		appevents.ConversationParticipantsChanged{Base: base(tc, ""), ConversationRef: ref},
	}
}
