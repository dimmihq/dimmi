/*
|--------------------------------------------------------------------------
| Reading Ratings
|--------------------------------------------------------------------------
|
| The team reads how customers rated a conversation beside it: each
| answered ask, newest first, with the teammate it rated. Only those who
| may see the conversation read its ratings.
|
*/

package ratings

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OfConversation returns the conversation's answered asks, newest first.
*/
func (r *Ratings) OfConversation(ctx context.Context, m authz.Membership, ref conversations.Ref,
) ([]models.ListConversationRatingsRow, error) {
	if err := conversations.CanRead(m); err != nil {
		return nil, err
	}

	var out []models.ListConversationRatingsRow
	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := conversations.Find(ctx, q, m, ref)
		if err != nil {
			return err
		}
		out, err = q.ListConversationRatings(ctx, models.ListConversationRatingsParams{
			WorkspaceID: m.Workspace.ID, ConversationID: c.ID,
		})
		if err != nil {
			return fmt.Errorf("list conversation ratings: %w", err)
		}
		return nil
	})

	return out, err
}
