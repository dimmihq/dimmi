/*
|--------------------------------------------------------------------------
| Show Conversation
|--------------------------------------------------------------------------
|
| One conversation with everything shown beside it, named by its id or
| by its number, the #1042 people quote to each other.
|
*/

package conversations

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ShowConversationInput is who asks about which conversation.
*/
type ShowConversationInput struct {
	Membership authz.Membership
	Ref        Ref
}

/*
ShowConversation returns one conversation.

Rules:
  - The caller needs conversations:read.
  - Deleted conversations and those of other workspaces are not found.
  - One merged into a conversation the caller sees answers
    [exceptions.ErrMergedInto] naming the target.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMergedInto], or a wrapped database error.
*/
type ShowConversation struct {
	DB *database.DB
}

/*
Handle returns the conversation.
*/
func (a *ShowConversation) Handle(ctx context.Context, in ShowConversationInput) (Loaded, error) {
	if err := CanRead(in.Membership); err != nil {
		return Loaded{}, err
	}

	var out Loaded

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := Find(ctx, q, in.Membership, in.Ref)
		if errors.Is(err, exceptions.ErrNotFound) {
			return mergedTarget(ctx, q, in.Membership, in.Ref)
		}

		if err != nil {
			return err
		}

		out, err = LoadOne(ctx, q, in.Membership.Member.ID, c)

		return err
	})

	return out, err
}
