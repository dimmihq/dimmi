/*
|--------------------------------------------------------------------------
| Delete Saved Replies
|--------------------------------------------------------------------------
|
| Removing a reply. A member removes their own; a shared one takes
| saved_replies:manage_shared.
|
*/

package savedreplies

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteSavedReplyInput is who removes which reply.
*/
type DeleteSavedReplyInput struct {
	Membership authz.Membership
	ID         ids.UUID
}

/*
DeleteSavedReply removes a reply.

Rules:
  - The caller may remove their own replies, and shared ones with
    saved_replies:manage_shared.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a wrapped
database error.
*/
type DeleteSavedReply struct {
	DB *database.DB
}

/*
Handle removes the reply.
*/
func (a *DeleteSavedReply) Handle(ctx context.Context, in DeleteSavedReplyInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := find(ctx, q, in.Membership, in.ID)
		if err != nil {
			return err
		}

		if !canChange(in.Membership, current.OwnerMemberID == nil) {
			return exceptions.ErrForbidden
		}

		if _, err := q.DeleteSavedReply(ctx, models.DeleteSavedReplyParams{
			WorkspaceID: current.WorkspaceID, ID: current.ID,
		}); err != nil {
			return fmt.Errorf("delete saved reply: %w", err)
		}

		return nil
	})
}
