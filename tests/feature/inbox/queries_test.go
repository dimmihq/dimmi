package inbox_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
query runs the function in its own transaction scoped to the workspace,
as the application role, so row level security applies, with the
queries bound to it, and returns what the function returned. Assertions
stay outside, so a failing test never leaves a transaction open.
*/
func query[T any](
	t *testing.T, tc *tests.TestCase, workspace ids.UUID, fn func(ctx context.Context, q *models.Queries) (T, error),
) (T, error) {
	t.Helper()

	var out T

	ctx := tenancy.With(t.Context(), workspace.String())
	err := tc.ApplicationRole().Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = fn(ctx, models.New(tx))

		return err
	})

	return out, err
}

/*
thread is an inbox with a widget connection and a contact, ready for
conversations.
*/
type thread struct {
	workspace  ids.UUID
	inbox      models.Inbox
	connection models.ChannelConnection
	contact    models.Contact
}

/*
seedThread creates an inbox, a widget connection and a contact in the
workspace.
*/
func seedThread(t *testing.T, tc *tests.TestCase, workspace ids.UUID) thread {
	t.Helper()
	now := tc.Clock.Now()

	th, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (thread, error) {
		th := thread{workspace: workspace}

		var err error
		th.inbox, err = q.CreateInbox(ctx, models.CreateInboxParams{
			WorkspaceID: workspace, Name: "Sales", Settings: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return th, err
		}

		th.connection, err = q.CreateChannelConnection(ctx, models.CreateChannelConnectionParams{
			WorkspaceID: workspace, InboxID: th.inbox.ID, Type: "widget", Name: "Website", Status: "active",
			Config: json.RawMessage(`{}`), Now: now,
		})
		if err != nil {
			return th, err
		}

		th.contact, err = q.CreateContact(ctx, models.CreateContactParams{
			WorkspaceID: workspace, Kind: string(enums.ContactKindUser), ExternalID: new("user-42"),
			Email: new("Ada@example.com"), Name: new("Ada"), CustomAttributes: json.RawMessage(`{"plan":"pro"}`),
			Now: now,
		})

		return th, err
	})
	require.NoError(t, err)

	return th
}

/*
startConversation allocates the workspace's next number and starts a
conversation on the thread's connection.
*/
func startConversation(t *testing.T, tc *tests.TestCase, th thread) models.Conversation {
	t.Helper()
	now := tc.Clock.Now()

	conversation, err := query(t, tc, th.workspace,
		func(ctx context.Context, q *models.Queries) (models.Conversation, error) {
			number, err := q.AllocateConversationNumber(ctx, th.workspace)
			if err != nil {
				return models.Conversation{}, err
			}

			return q.CreateConversation(ctx, models.CreateConversationParams{
				WorkspaceID: th.workspace, Number: number, InboxID: th.inbox.ID,
				ChannelConnectionID: th.connection.ID, ContactID: th.contact.ID, Subject: new("Refund"),
				Status: string(enums.ConversationStatusOpen), Source: json.RawMessage(`{}`), Now: now,
			})
		})
	require.NoError(t, err)

	return conversation
}

/*
message returns a customer message in the conversation with the body.
*/
func message(c models.Conversation, body string, now time.Time) models.CreateMessageParams {
	return models.CreateMessageParams{
		WorkspaceID: c.WorkspaceID, ConversationID: c.ID, Type: string(enums.MessageTypeMessage),
		AuthorType: string(enums.AuthorTypeContact), AuthorID: &c.ContactID, Body: &body,
		Substantive: true, Metadata: json.RawMessage(`{}`), Now: now,
	}
}

/*
appendMessage appends the message to its conversation.
*/
func appendMessage(t *testing.T, tc *tests.TestCase, params models.CreateMessageParams) (models.Message, error) {
	t.Helper()

	return query(t, tc, params.WorkspaceID, func(ctx context.Context, q *models.Queries) (models.Message, error) {
		return q.CreateMessage(ctx, params)
	})
}

/*
listConversations walks every page of the workspace's conversations,
the given number at a time, and returns their numbers in order.
*/
func listConversations(t *testing.T, tc *tests.TestCase, workspace ids.UUID, size int32) []int64 {
	t.Helper()

	var numbers []int64

	params := models.ListConversationsParams{WorkspaceID: workspace, RowLimit: size}
	for {
		page, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) ([]models.Conversation, error) {
				return q.ListConversations(ctx, params)
			})
		require.NoError(t, err)

		if len(page) == 0 {
			return numbers
		}

		for _, c := range page {
			numbers = append(numbers, c.Number)
		}

		last := page[len(page)-1]
		params.BeforeID, params.BeforeCreatedAt = &last.ID, &last.CreatedAt
	}
}

/*
listMessages returns one page of the conversation's timeline, after the
message when one is given.
*/
func listMessages(t *testing.T, tc *tests.TestCase, c models.Conversation, after *models.Message) []models.Message {
	t.Helper()

	params := models.ListMessagesParams{WorkspaceID: c.WorkspaceID, ConversationID: c.ID, RowLimit: 2}
	if after != nil {
		params.AfterID, params.AfterCreatedAt = &after.ID, &after.CreatedAt
	}

	page, err := query(t, tc, c.WorkspaceID, func(ctx context.Context, q *models.Queries) ([]models.Message, error) {
		return q.ListMessages(ctx, params)
	})
	require.NoError(t, err)

	return page
}

func TestInboxQueries(t *testing.T) {
	t.Parallel()

	t.Run("creates an inbox and finds it in its workspace only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		th := seedThread(t, tc, workspace)

		found, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Inbox, error) {
			return q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: workspace, ID: th.inbox.ID})
		})
		require.NoError(t, err)
		listed, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]models.Inbox, error) {
			return q.ListInboxes(ctx, workspace)
		})
		require.NoError(t, err)
		_, err = query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Inbox, error) {
			return q.FindInbox(ctx, models.FindInboxParams{WorkspaceID: ids.New(), ID: th.inbox.ID})
		})

		assert.Equal(t, "Sales", found.Name)
		require.Len(t, listed, 2)
		assert.Equal(t, "Support", listed[0].Name)
		assert.Equal(t, found, listed[1])
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("refuses a provider key another live connection holds", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		th := seedThread(t, tc, workspace)
		params := models.CreateChannelConnectionParams{
			WorkspaceID: workspace, InboxID: th.inbox.ID, Type: "email", Name: "Support mail", Status: "pending",
			ExternalID: new("help@acme.test"), Config: json.RawMessage(`{}`), Now: tc.Clock.Now(),
		}
		connect := func(ctx context.Context, q *models.Queries) (models.ChannelConnection, error) {
			return q.CreateChannelConnection(ctx, params)
		}

		first, err := query(t, tc, workspace, connect)
		require.NoError(t, err)
		found, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) (models.ChannelConnection, error) {
				return q.FindChannelConnection(ctx, models.FindChannelConnectionParams{
					WorkspaceID: workspace, ID: first.ID,
				})
			})
		require.NoError(t, err)
		_, err = query(t, tc, workspace, connect)

		assert.Equal(t, first, found)
		assert.True(t, database.IsUniqueViolation(err, "channel_connections_type_external_id_key"))
	})

	t.Run("finds a contact by id, by external id and by any of its identities", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		th := seedThread(t, tc, workspace)

		identity, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) (models.ContactIdentity, error) {
				return q.CreateContactIdentity(ctx, models.CreateContactIdentityParams{
					WorkspaceID: workspace, ContactID: th.contact.ID, Type: "email", Value: "ada@example.com",
					Verified: true, Now: tc.Clock.Now(),
				})
			})
		require.NoError(t, err)
		found, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) ([]models.Contact, error) {
			byID, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: workspace, ID: th.contact.ID})
			if err != nil {
				return nil, err
			}

			byExternal, err := q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
				WorkspaceID: workspace, ExternalID: new("user-42"),
			})
			if err != nil {
				return nil, err
			}

			byIdentity, err := q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
				WorkspaceID: workspace, Type: "email", Value: "ada@example.com",
			})

			return []models.Contact{byID, byExternal, byIdentity}, err
		})
		require.NoError(t, err)
		_, err = query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Contact, error) {
			return q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
				WorkspaceID: workspace, Type: "phone", Value: "ada@example.com",
			})
		})

		assert.True(t, identity.Verified)
		assert.Equal(t, []models.Contact{th.contact, th.contact, th.contact}, found)
		assert.Equal(t, []string{}, th.contact.TrustedAttributeKeys)
		assert.JSONEq(t, `{"plan":"pro"}`, string(th.contact.CustomAttributes))
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("keeps one contact per external id and per identity in a workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		th := seedThread(t, tc, workspace)
		identify := func(ctx context.Context, q *models.Queries) (models.ContactIdentity, error) {
			return q.CreateContactIdentity(ctx, models.CreateContactIdentityParams{
				WorkspaceID: workspace, ContactID: th.contact.ID, Type: "widget_visitor", Value: "v1",
				Now: tc.Clock.Now(),
			})
		}

		_, err := query(t, tc, workspace, identify)
		require.NoError(t, err)
		_, duplicateIdentity := query(t, tc, workspace, identify)
		_, duplicateContact := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) (models.Contact, error) {
				return q.CreateContact(ctx, models.CreateContactParams{
					WorkspaceID: workspace, Kind: string(enums.ContactKindUser), ExternalID: new("user-42"),
					CustomAttributes: json.RawMessage(`{}`), Now: tc.Clock.Now(),
				})
			})

		assert.True(t, database.IsUniqueViolation(duplicateContact, "contacts_workspace_id_external_id_key"))
		assert.True(t, database.IsUniqueViolation(duplicateIdentity, "contact_identities_workspace_id_type_value_key"))
	})

	t.Run("numbers conversations one after another within each workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		ours, theirs := seedThread(t, tc, tc.Workspace.Workspace.ID), seedThread(t, tc, other.Workspace.ID)

		numbers := []int64{
			startConversation(t, tc, ours).Number, startConversation(t, tc, ours).Number,
			startConversation(t, tc, theirs).Number, startConversation(t, tc, ours).Number,
		}

		assert.Equal(t, []int64{1, 2, 1, 3}, numbers)
	})

	t.Run("finds a conversation in its workspace and nowhere else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		conversation := startConversation(t, tc, seedThread(t, tc, workspace))
		find := func(in ids.UUID) (models.Conversation, error) {
			return query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Conversation, error) {
				return q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: in, ID: conversation.ID})
			})
		}

		found, err := find(workspace)
		require.NoError(t, err)
		_, err = find(ids.New())

		assert.Equal(t, conversation, found)
		assert.Equal(t, "open", found.Status)
		assert.Equal(t, "off", found.AiState)
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("lists conversations newest first, a page at a time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		th := seedThread(t, tc, workspace)
		for range 5 {
			startConversation(t, tc, th)
			tc.Clock.Travel(time.Minute)
		}

		assert.Equal(t, []int64{5, 4, 3, 2, 1}, listConversations(t, tc, workspace, 2))
		assert.Equal(t, []int64{5, 4, 3, 2, 1}, listConversations(t, tc, workspace, 10))
	})

	t.Run("appends messages and lists the timeline oldest first, a page at a time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		conversation := startConversation(t, tc, seedThread(t, tc, tc.Workspace.Workspace.ID))
		for _, body := range []string{"Hello", "I need a refund", "Thanks"} {
			_, err := appendMessage(t, tc, message(conversation, body, tc.Clock.Now()))
			require.NoError(t, err)
		}

		first := listMessages(t, tc, conversation, nil)
		require.Len(t, first, 2)
		rest := listMessages(t, tc, conversation, &first[1])
		require.Len(t, rest, 1)

		assert.Equal(t, []string{"Hello", "I need a refund", "Thanks"},
			[]string{*first[0].Body, *first[1].Body, *rest[0].Body})
		assert.True(t, rest[0].Search.Valid)
		assert.True(t, rest[0].Substantive)
	})

	t.Run("accepts an inbound message once however often the channel retries it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		conversation := startConversation(t, tc, seedThread(t, tc, tc.Workspace.Workspace.ID))
		inbound := message(conversation, "Hello", tc.Clock.Now())
		inbound.DedupeKey = new("widget:abc")

		_, err := appendMessage(t, tc, inbound)
		require.NoError(t, err)
		_, err = appendMessage(t, tc, inbound)

		assert.True(t, database.IsUniqueViolation(err, "messages_workspace_id_dedupe_key_key"))
	})

	t.Run("adds a participant once and lists participants in the order they joined", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		th := seedThread(t, tc, workspace)
		conversation := startConversation(t, tc, th)
		add := func(role enums.ParticipantRole) (models.ConversationParticipant, error) {
			return query(t, tc, workspace,
				func(ctx context.Context, q *models.Queries) (models.ConversationParticipant, error) {
					return q.AddConversationParticipant(ctx, models.AddConversationParticipantParams{
						WorkspaceID: workspace, ConversationID: conversation.ID, ContactID: th.contact.ID,
						Role: string(role), Now: tc.Clock.Now(),
					})
				})
		}

		added, err := add(enums.ParticipantRoleCC)
		require.NoError(t, err)
		again, err := add(enums.ParticipantRoleParticipant)
		require.NoError(t, err)
		listed, err := query(t, tc, workspace,
			func(ctx context.Context, q *models.Queries) ([]models.ConversationParticipant, error) {
				return q.ListConversationParticipants(ctx, models.ListConversationParticipantsParams{
					WorkspaceID: workspace, ConversationID: conversation.ID,
				})
			})
		require.NoError(t, err)

		assert.Equal(t, added, again)
		assert.Equal(t, []models.ConversationParticipant{added}, listed)
		assert.Equal(t, "cc", added.Role)
	})
}
