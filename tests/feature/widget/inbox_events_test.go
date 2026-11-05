package widget_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
inboxReply has the owner answer and returns the conversation's and the
reply's ids.
*/
func inboxReply(t *testing.T, tc *tests.TestCase, conversation string) (ids.UUID, ids.UUID) {
	t.Helper()

	public := owner(tc).PostJSON("/api/v1/conversations/"+conversation+"/replies", testkit.JSON{"body": "on it"}).
		AssertCreated().String("data.id")

	conv, err := tc.App.IDs.Decode(string(models.PrefixConversation), conversation)
	require.NoError(t, err)
	msg, err := tc.App.IDs.Decode(string(models.PrefixMessage), public)
	require.NoError(t, err)

	return conv, msg
}

/*
dispatch sends an event the inbox records, as its own actions do.
*/
func dispatch(t *testing.T, tc *tests.TestCase, e events.Event) {
	t.Helper()

	ctx := tenancy.With(t.Context(), tc.Workspace.Workspace.ID.String())
	err := tc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return tc.App.Events.Dispatch(ctx, tx, e)
	})
	require.NoError(t, err)
}

func TestWidgetInboxEvents(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*tests.TestCase, *stream, appevents.Base, appevents.MessageRef) {
		t.Helper()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		conv, msg := inboxReply(t, tc, id)

		s := v.openStream(t, streamURL(t, tc), nil)
		require.NotNil(t, s.next().Connect)

		base := appevents.Base{ID: ids.New(), WorkspaceID: tc.Workspace.Workspace.ID, OccurredAt: tc.Clock.Now()}
		ref := appevents.MessageRef{ConversationRef: appevents.ConversationRef{ConversationID: conv}, MessageID: msg}

		return tc, s, base, ref
	}

	t.Run("an edited reply reaches the visitor", func(t *testing.T) {
		t.Parallel()
		tc, s, base, ref := setup(t)

		dispatch(t, tc, appevents.MessageUpdated{Base: base, MessageRef: ref})

		event, data := s.eventOf("message.updated")
		assert.Equal(t, "message.updated", event)
		assert.Contains(t, string(data), "on it")
	})

	t.Run("a reply the team deletes reaches the visitor without its text", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		_, msg := inboxReply(t, tc, id)
		s := v.openStream(t, streamURL(t, tc), nil)
		require.NotNil(t, s.next().Connect)

		owner(tc).Delete("/api/v1/conversations/" + id + "/messages/" + models.PublicID(models.PrefixMessage, msg)).
			AssertOK()

		_, data := s.eventOf("message.deleted")
		assert.NotContains(t, string(data), "on it")
	})

	t.Run("a delivery update naming one message reaches the visitor", func(t *testing.T) {
		t.Parallel()
		tc, s, base, ref := setup(t)

		dispatch(t, tc, appevents.MessageDeliveryUpdated{Base: base, MessageRef: ref, Status: "read"})

		_, data := s.eventOf("message.delivery_updated")
		assert.Contains(t, string(data), models.PublicID(models.PrefixMessage, ref.MessageID))
	})
}

func TestWidgetInboxEventsWithoutPayload(t *testing.T) {
	t.Parallel()

	event := func(tc *tests.TestCase, conversation, message ids.UUID) (appevents.Base, appevents.MessageRef) {
		base := appevents.Base{ID: ids.New(), WorkspaceID: tc.Workspace.Workspace.ID, OccurredAt: tc.Clock.Now()}

		return base, appevents.MessageRef{
			ConversationRef: appevents.ConversationRef{ConversationID: conversation}, MessageID: message,
		}
	}

	t.Run("events about rows that are gone publish nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		base, ref := event(tc, ids.New(), ids.New())

		dispatch(t, tc, appevents.MessageDeliveryUpdated{Base: base, MessageRef: ref, Status: "read"})
		dispatch(t, tc, appevents.ConversationClosed{Base: base, ConversationRef: ref.ConversationRef})
		dispatch(t, tc, appevents.MessageUpdated{Base: base, MessageRef: ref})
	})

	for name, table := range map[string]string{
		"conversations": "conversations", "channels": "channel_connections",
	} {
		t.Run("a change whose "+name+" cannot be read is logged and lets the change stand", func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)
			v := startVisitor(t, tc)
			conv, msg := inboxReply(t, tc, v.startConversation(t, "hello"))
			base, ref := event(tc, conv, msg)
			tc.BreakTable(table)

			dispatch(t, tc, appevents.MessageUpdated{Base: base, MessageRef: ref})
		})
	}
}
