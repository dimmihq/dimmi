/*
|--------------------------------------------------------------------------
| Create Test Conversations
|--------------------------------------------------------------------------
|
| A new workspace can start with five made-up customers writing in, each
| with a different kind of request, so its team practices replying,
| tagging and closing before the first real conversation arrives. They
| arrive the way a real customer's message does, through the default
| inbox, and are marked as tests: never billed, never sent anywhere.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/auth/authz"
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
  - The conversations land in the workspace's default inbox.

Errors: [exceptions.ErrForbidden], [exceptions.ErrInboxNotFound].
*/
type CreateTestConversations struct {
	DB     *database.DB
	Create *CreateConversation
}

/*
Handle creates the conversations and marks each one as a test.
*/
func (a *CreateTestConversations) Handle(ctx context.Context, membership authz.Membership) ([]Loaded, error) {
	if err := CanWrite(membership); err != nil {
		return nil, err
	}

	inbox, err := a.defaultInbox(ctx, membership)
	if err != nil {
		return nil, err
	}

	created := make([]Loaded, 0, len(testConversations()))

	for _, sample := range testConversations() {
		loaded, err := a.start(ctx, membership, inbox, sample)
		if err != nil {
			return created, err
		}

		created = append(created, loaded)
	}

	return created, nil
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
) (Loaded, error) {
	ws := membership.Workspace.ID

	loaded, err := a.Create.Handle(ctx, CreateConversationInput{
		Membership: membership, ByContact: true, InboxID: &inbox.ID,
		Contact: contacts.Ref{WorkspaceID: ws, Email: &sample.email, Name: &sample.name},
		Subject: &sample.subject, Body: sample.body,
	})
	if err != nil {
		return Loaded{}, fmt.Errorf("start test conversation: %w", err)
	}

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).MarkConversationAsTest(ctx, models.MarkConversationAsTestParams{
			WorkspaceID: ws, ID: loaded.Conversation.ID,
		})
	})
	if err != nil {
		return Loaded{}, fmt.Errorf("mark test conversation: %w", err)
	}

	loaded.Conversation.IsTest = true

	return loaded, nil
}
