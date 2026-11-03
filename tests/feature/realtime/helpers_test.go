package realtime_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/database"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
query runs the function in a transaction scoped to the workspace and
returns what it returned.
*/
func query[T any](
	t *testing.T, tc *tests.TestCase, workspace ids.UUID, fn func(ctx context.Context, q *models.Queries) (T, error),
) T {
	t.Helper()

	var out T

	err := tc.App.DB.Transaction(tenancy.With(t.Context(), workspace.String()),
		func(ctx context.Context, tx *database.Tx) error {
			var err error
			out, err = fn(ctx, models.New(tx))

			return err
		})
	require.NoError(t, err)

	return out
}

/*
exec runs a statement inside the test case's workspace.
*/
func exec(t *testing.T, tc *tests.TestCase, sql string, args ...any) {
	t.Helper()

	err := tc.App.DB.Transaction(tenancy.With(t.Context(), tc.Workspace.Workspace.ID.String()),
		func(ctx context.Context, tx *database.Tx) error {
			_, err := tx.Exec(ctx, sql, args...)

			return err
		})
	require.NoError(t, err)
}

/*
seed is a conversation with its inbox, channel and contact.
*/
type seed struct {
	connection   models.ChannelConnection
	contact      models.Contact
	conversation models.Conversation
}

/*
startConversation starts a conversation on a new channel of the type in
the workspace, with a contact of its own.
*/
func startConversation(t *testing.T, tc *tests.TestCase, workspace ids.UUID, channelType string) seed {
	t.Helper()
	now := tc.Clock.Now()

	return query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (seed, error) {
		var s seed

		inbox, err := q.CreateInbox(ctx, models.CreateInboxParams{
			WorkspaceID: workspace, Name: "Support", Settings: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return s, err
		}

		s.connection, err = q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
			WorkspaceID: workspace, InboxID: inbox.ID, Type: channelType, Name: "Website", Status: "active",
			ExternalID: new(testkit.UniqueName(channelType)), Config: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return s, err
		}

		s.contact, err = q.CreateContact(ctx, models.CreateContactParams{
			WorkspaceID: workspace, Kind: string(enums.ContactKindUser), Name: new("Bob Lee"),
			CustomAttributes: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return s, err
		}

		number, err := q.AllocateConversationNumber(ctx, workspace)
		if err != nil {
			return s, err
		}

		s.conversation, err = q.CreateConversation(ctx, models.CreateConversationParams{
			WorkspaceID: workspace, Number: number, InboxID: inbox.ID, ChannelConnectionID: s.connection.ID,
			ContactID: s.contact.ID, Subject: new("Card declined"), Status: string(enums.ConversationStatusOpen),
			Source: json.RawMessage(`{}`), Now: now,
		})

		return s, err
	})
}

/*
addMessage appends a customer message to the conversation.
*/
func addMessage(t *testing.T, tc *tests.TestCase, c models.Conversation, body string) models.Message {
	t.Helper()

	return query(t, tc, c.WorkspaceID, func(ctx context.Context, q *models.Queries) (models.Message, error) {
		return q.CreateMessage(ctx, models.CreateMessageParams{
			WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Type: string(enums.MessageTypeMessage),
			AuthorType: string(enums.AuthorTypeContact), AuthorID: &c.ContactID, Body: &body,
			Substantive: true, Metadata: json.RawMessage(`{}`), Now: tc.Clock.Now(),
		})
	})
}

/*
dispatch dispatches the event outside a transaction, so its broadcast
goes out at once.
*/
func dispatch(t *testing.T, tc *tests.TestCase, e any) {
	t.Helper()

	require.NoError(t, tc.App.Events.Dispatch(t.Context(), nil, e))
}

/*
base is the header of an event in the test case's workspace, with the
client mutation id.
*/
func base(tc *tests.TestCase, mutation string) appevents.Base {
	return appevents.Base{
		ID: ids.New(), WorkspaceID: tc.Workspace.Workspace.ID, OccurredAt: tc.Clock.Now(),
		Actor:            appevents.Actor{Type: "member", ID: tc.Workspace.Owner.ID},
		ClientMutationID: mutation,
	}
}

/*
joinAsMember adds a new user to the workspace as a plain member.
*/
func joinAsMember(t *testing.T, tc *tests.TestCase) (models.User, models.WorkspaceMember) {
	t.Helper()

	user := tc.CreateUser()
	member, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
	})
	require.NoError(t, err)

	return user, member
}

/*
otherWorkspace creates a workspace the test case's owner is not in and
returns its owner.
*/
func otherWorkspace(t *testing.T, tc *tests.TestCase) (models.User, ids.UUID) {
	t.Helper()

	owner := tc.CreateUser()
	created, err := factories.Workspace(t.Context(), tc.App, owner)
	require.NoError(t, err)

	return owner, created.Workspace.ID
}

/*
secondNode starts another realtime node of the application on the same
Valkey, with the application's channels and the resolver given, and
returns its base URL.
*/
func secondNode(t *testing.T, tc *tests.TestCase, resolve broadcasting.Resolver) string {
	t.Helper()

	logger, err := log.New(log.Options{Driver: "json", Level: "error", Writer: io.Discard})
	require.NoError(t, err)

	server, err := broadcasting.Open(log.NewContext(t.Context(), logger), tc.App.Redis,
		broadcasting.Config{Resolve: resolve})
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Shutdown(context.WithoutCancel(t.Context())) })

	authorize := &appbroadcasting.Authorizers{
		DB: tc.App.DB, IDs: tc.App.IDs, Resolve: tc.App.Actions.ResolveWorkspace,
		Scope: appbroadcasting.WorkspaceScope{},
	}
	channels := server.Channels()
	channels.MustAuthorize(appbroadcasting.WorkspacePattern, appbroadcasting.WorkspaceOptions(), authorize.Workspace)
	channels.MustAuthorize(appbroadcasting.ConversationPattern, appbroadcasting.ConversationOptions(),
		authorize.Conversation)
	channels.MustAuthorize(appbroadcasting.MemberPattern, appbroadcasting.MemberOptions(), authorize.Member)
	channels.MustAuthorize(appbroadcasting.WidgetContactPattern, appbroadcasting.WidgetContactOptions(),
		authorize.WidgetContact)

	return testkit.Serve(t, server.Handlers()[broadcasting.PathWebsocket])
}

/*
widgetNode starts a second node whose connections are messenger widgets
of the contact, as the widget's own connect tokens will make them.
*/
func widgetNode(t *testing.T, tc *tests.TestCase, contact models.Contact) string {
	t.Helper()

	return secondNode(t, tc, func(context.Context, *fhttp.Request, string) (broadcasting.Principal, error) {
		return broadcasting.Principal{
			UserID:  models.PublicID(models.PrefixContact, contact.ID),
			Subject: &appbroadcasting.WidgetSession{WorkspaceID: contact.WorkspaceID, ContactID: contact.ID},
		}, nil
	})
}

/*
workspaceID returns the public id of the test case's workspace.
*/
func workspaceID(tc *tests.TestCase) string {
	return public(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
}

/*
unknownToken returns a token of the right shape that no one minted.
*/
func unknownToken() string {
	return "dmi_live_" + ids.New().String()
}

/*
public returns the public id of the row.
*/
func public(prefix models.Prefix, id ids.UUID) string {
	return models.PublicID(prefix, id)
}
