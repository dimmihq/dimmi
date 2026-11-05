/*
|--------------------------------------------------------------------------
| Create Test Conversations
|--------------------------------------------------------------------------
|
| A new workspace can start with five made-up customers writing in, each
| with a different kind of request, so its team practices replying,
| tagging and closing before the first real conversation arrives. They
| arrive the way a real customer's message does, through the workspace's
| messenger, the channel setup recommends, or through the default inbox
| when it has none, and are marked as tests: never billed, never sent
| anywhere.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
testConversation is one made-up customer and what they write.
*/
type testConversation struct {
	name, email, subject, body string
}

/*
testConversations are the made-up customers a new workspace practices on.
Their addresses are on example.com, which never receives mail.
*/
func testConversations() []testConversation {
	return []testConversation{
		{
			"Emily Johnson", "emily.johnson@example.com", "Can't sign in",
			"I keep landing back on the sign-in page after typing my password. Is something wrong with my account?",
		},
		{
			"David Smith", "david.smith@example.com", "API requests failing with 401",
			"Our calls to your API started failing with 401 this morning. We didn't change our key. Can you check?",
		},
		{
			"Maria Garcia", "maria.garcia@example.com", "Changing my username",
			"Where do I change the username on my profile? I can't find the setting anywhere.",
		},
		{
			"James Wilson", "james.wilson@example.com", "Charged twice this month",
			"I can see two charges on my card for this month's invoice. Could you refund one of them?",
		},
		{
			"Sophia Martinez", "sophia.martinez@example.com", "Upgrading our plan",
			"We'd like to move the team to a bigger plan. What changes for us when we upgrade?",
		},
	}
}

/*
CreateTestConversations starts the practice conversations of a workspace.

Rules:
  - The caller needs conversations:write.
  - The conversations come in through the workspace's first messenger,
    or the default inbox's API channel without one.

Errors: [exceptions.ErrForbidden], [exceptions.ErrInboxNotFound].
*/
type CreateTestConversations struct {
	DB       *database.DB
	Clock    clock.Clock
	Create   *CreateConversation
	Receiver *ReceiveInboundMessage
}

/*
Handle creates the conversations, marks each one as a test and returns
how many it started.
*/
func (a *CreateTestConversations) Handle(ctx context.Context, membership authz.Membership) (int, error) {
	if err := CanWrite(membership); err != nil {
		return 0, err
	}

	messenger, err := a.messenger(ctx, membership.Workspace.ID)
	if err != nil {
		return 0, err
	}

	if messenger == nil {
		return a.throughInbox(ctx, membership)
	}

	for i, sample := range testConversations() {
		if err := a.receive(ctx, membership.Workspace.ID, *messenger, sample); err != nil {
			return i, err
		}
	}

	return len(testConversations()), nil
}

/*
messenger finds the connection of the workspace's first messenger, or
nil when it has none.
*/
func (a *CreateTestConversations) messenger(ctx context.Context, ws ids.UUID) (*ids.UUID, error) {
	var found *ids.UUID

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		widgets, err := models.New(tx).ListWidgetSettings(ctx, ws)
		if err != nil {
			return fmt.Errorf("list messengers: %w", err)
		}

		if len(widgets) > 0 {
			found = &widgets[0].ChannelConnectionID
		}

		return nil
	})

	return found, err
}

/*
receive takes one made-up customer's message in through the messenger,
as a visitor's would arrive, and marks its conversation as a test.
*/
func (a *CreateTestConversations) receive(
	ctx context.Context, ws, connection ids.UUID, sample testConversation,
) error {
	result, err := a.Receiver.ReceiveInboundMessage(ctx, contracts.InboundMessage{
		ConnectionID: connection,
		DedupeKey:    "example:" + sample.email,
		Sender: contracts.InboundSender{
			Name: sample.name, Email: sample.email, Kind: enums.ContactKindLead,
			Identities: []contracts.IdentityClaim{{Type: enums.IdentityTypeEmail, Value: sample.email}},
		},
		Subject: sample.subject, Body: sample.body, SentAt: a.Clock.Now(), IsTest: true,
	})
	if err != nil {
		return fmt.Errorf("receive test conversation: %w", err)
	}

	return a.markTest(ctx, ws, result.ConversationID)
}

/*
throughInbox starts the conversations on the default inbox's API
channel, for a workspace without a messenger.
*/
func (a *CreateTestConversations) throughInbox(ctx context.Context, membership authz.Membership) (int, error) {
	inbox, err := a.defaultInbox(ctx, membership)
	if err != nil {
		return 0, err
	}

	for i, sample := range testConversations() {
		if err := a.start(ctx, membership, inbox, sample); err != nil {
			return i, err
		}
	}

	return len(testConversations()), nil
}

func (a *CreateTestConversations) defaultInbox(ctx context.Context, membership authz.Membership) (models.Inbox, error) {
	var inbox models.Inbox

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		inbox, err = models.New(tx).FindDefaultInbox(ctx, membership.Workspace.ID)
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrInboxNotFound
		}

		if err != nil {
			return fmt.Errorf("find default inbox: %w", err)
		}

		return nil
	})

	return inbox, err
}

func (a *CreateTestConversations) start(
	ctx context.Context, membership authz.Membership, inbox models.Inbox, sample testConversation,
) error {
	ws := membership.Workspace.ID

	loaded, err := a.Create.Handle(ctx, CreateConversationInput{
		Membership: membership, ByContact: true, InboxID: &inbox.ID,
		Contact: contacts.Ref{WorkspaceID: ws, Email: &sample.email, Name: &sample.name},
		Subject: &sample.subject, Body: sample.body,
	})
	if err != nil {
		return fmt.Errorf("start test conversation: %w", err)
	}

	return a.markTest(ctx, ws, loaded.Conversation.ID)
}

/*
markTest marks the conversation as a practice one.
*/
func (a *CreateTestConversations) markTest(ctx context.Context, ws, conversation ids.UUID) error {
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).MarkConversationAsTest(ctx, models.MarkConversationAsTestParams{
			WorkspaceID: ws, ID: conversation,
		})
	})
	if err != nil {
		return fmt.Errorf("mark test conversation: %w", err)
	}

	return nil
}
