/*
|--------------------------------------------------------------------------
| Merged Conversations
|--------------------------------------------------------------------------
|
| A conversation merged into another is in the trash and points at the
| one it went into. A link to it should still lead somewhere, so asking
| for it answers with where it went, as long as the caller may see that
| conversation. A chain of merges is followed to its end.
|
*/

package conversations

import (
	"context"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxMergeHops is how many merges are followed before giving up, which
only a merge loop would ever reach.
*/
const maxMergeHops = 10

/*
mergedTarget answers a conversation that was not found:
[exceptions.ErrMergedInto] naming the live conversation it was merged
into, or [exceptions.ErrNotFound] when it was not merged or the caller
cannot see where it went.
*/
func mergedTarget(ctx context.Context, q *models.Queries, m authz.Membership, ref Ref) error {
	c, err := findAny(ctx, q, m, ref)
	if err != nil {
		return exceptions.ErrNotFound
	}

	for range maxMergeHops {
		if c.MergedIntoID == nil {
			return exceptions.ErrNotFound
		}

		next, err := findAny(ctx, q, m, Ref{ID: *c.MergedIntoID})
		if err != nil {
			return exceptions.ErrNotFound
		}

		if next.DeletedAt == nil {
			if _, err := visible(ctx, q, m, next); err != nil {
				return exceptions.ErrNotFound
			}

			return exceptions.ErrMergedInto.
				With("target_id", models.PublicID(models.PrefixConversation, next.ID)).
				With("target_number", next.Number)
		}

		c = next
	}

	return exceptions.ErrNotFound
}

/*
findAny returns the workspace's conversation the reference names,
trashed or not.
*/
func findAny(ctx context.Context, q *models.Queries, m authz.Membership, ref Ref) (models.Conversation, error) {
	if ref.Number > 0 {
		return q.FindConversationByNumber(ctx, models.FindConversationByNumberParams{
			WorkspaceID: m.Workspace.ID, Number: ref.Number,
		})
	}

	return q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: m.Workspace.ID, ID: ref.ID})
}
