package realtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
internal is the protocol's error code for a failure on the server.
*/
const internal = 100

/*
brokenDisconnector cannot reach the other nodes, as when Valkey is down.
*/
type brokenDisconnector struct{}

func (brokenDisconnector) Disconnect(context.Context, string) error {
	return errors.New("valkey is down")
}

func (brokenDisconnector) DisconnectCredential(context.Context, string, string) error {
	return errors.New("valkey is down")
}

func TestRealtimeFailures(t *testing.T) {
	t.Parallel()

	t.Run("does not undo what ended access when a connection cannot be closed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		dispatcher := events.New(tc.App.Queue)
		(&appbroadcasting.Disconnects{Server: brokenDisconnector{}}).Listen(dispatcher)

		assert.NoError(t, dispatcher.Dispatch(t.Context(), nil, appevents.UserSignedOutEverywhere{UserID: tc.Owner.ID}))
		assert.NoError(t, dispatcher.Dispatch(t.Context(), nil, appevents.SessionRevoked{UserID: tc.Owner.ID}))
	})

	t.Run("answers that a first-party app's grant is checked elsewhere", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		alive, err := tc.App.Realtime.Live.Alive(t.Context(), &appauth.Principal{User: tc.Owner, Kind: appauth.KindApp})

		require.NoError(t, err)
		assert.True(t, alive)
	})

	t.Run("cannot tell whether a credential is alive while its tables are down", func(t *testing.T) {
		t.Parallel()

		for _, broken := range []struct {
			table string
			kind  string
		}{
			{"sessions", appauth.KindSession},
			{"personal_access_tokens", appauth.KindToken},
			{"users", appauth.KindSession},
		} {
			tc := tests.New(t)
			principal := &appauth.Principal{
				User: tc.Owner, Kind: broken.kind, CredentialID: ids.New(), WorkspaceID: tc.Workspace.Workspace.ID,
			}
			tc.BreakTable(broken.table)

			_, err := tc.App.Realtime.Live.Alive(t.Context(), principal)

			assert.Error(t, err)
		}
	})

	t.Run("calls a token no row knows dead, and a deleted account's credential too", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := joinAsMember(t, tc)
		principal := &appauth.Principal{
			User: user, Kind: appauth.KindToken, CredentialID: ids.New(), WorkspaceID: tc.Workspace.Workspace.ID,
		}

		alive, err := tc.App.Realtime.Live.Alive(t.Context(), principal)
		require.NoError(t, err)
		assert.False(t, alive)

		require.NoError(t, factories.DeleteAccount(t.Context(), tc.App, user))
		alive, err = tc.App.Realtime.Live.Alive(t.Context(), principal)
		require.NoError(t, err)
		assert.False(t, alive)
	})

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
		require.Nil(t, c.Subscribe(appbroadcasting.ConversationChannel(s.conversation.ID)).Error)

		tc.BreakTable("messages")
		dispatch(t, tc, appevents.MessageCreated{
			Base: base(tc, ""), MessageRef: messageRef(s.conversation.ID, s.conversation.ID),
		})
		dispatch(t, tc, appevents.ContactTyping{
			Base: base(tc, ""), ConversationRef: appevents.ConversationRef{ConversationID: s.conversation.ID},
			ContactID: s.contact.ID, Started: true,
		})

		assert.Equal(t, "typing.started", c.Publication().Event, "the failed broadcast is skipped, not retried")
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
