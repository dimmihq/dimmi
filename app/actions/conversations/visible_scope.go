/*
|--------------------------------------------------------------------------
| Visible Scope
|--------------------------------------------------------------------------
|
| Every read and write of a conversation on someone's behalf goes through
| what that someone may see. Lists pass the scope to their query; a single
| conversation is looked up and then checked. Anything outside the scope
| is simply not found, never refused, so nobody learns a conversation
| exists by asking for it.
|
*/

package conversations

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Scope is what one caller sees of the workspace's conversations.
*/
type Scope = authz.ConversationScope

/*
VisibleParams are a scope's arguments to dimmi_conversation_visible.
*/
type VisibleParams = authz.VisibleParams

/*
VisibleScope returns what the caller sees. Resolving the workspace reads
the caller's list of teams once per request, so calling this costs no
query.
*/
func VisibleScope(m authz.Membership) Scope {
	return m.ConversationScope()
}

/*
Find returns the live conversation the reference names, or
exceptions.ErrNotFound when it does not exist or the caller may not see
it.
*/
func Find(ctx context.Context, q *models.Queries, m authz.Membership, ref Ref) (models.Conversation, error) {
	var (
		c   models.Conversation
		err error
	)

	if ref.ID == ids.Nil() {
		c, err = q.FindConversationByNumber(ctx, models.FindConversationByNumberParams{
			WorkspaceID: m.Workspace.ID, Number: ref.Number,
		})
	} else {
		c, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: m.Workspace.ID, ID: ref.ID})
	}

	if err != nil {
		return c, missing(err)
	}

	return visible(ctx, q, m, c)
}

/*
Lock returns the live conversation the reference names, when the caller
may see it, and holds its row until the transaction ends.
*/
func Lock(ctx context.Context, q *models.Queries, m authz.Membership, ref Ref) (models.Conversation, error) {
	found, err := Find(ctx, q, m, ref)
	if err != nil {
		return found, err
	}

	locked, err := q.LockConversation(ctx, models.LockConversationParams{WorkspaceID: m.Workspace.ID, ID: found.ID})
	if err != nil {
		return locked, missing(err)
	}

	return visible(ctx, q, m, locked)
}

/*
missing turns a conversation that is not there into
exceptions.ErrNotFound.
*/
func missing(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return exceptions.ErrNotFound
	}

	return fmt.Errorf("find conversation: %w", err)
}

/*
visible turns a deleted or hidden conversation into
exceptions.ErrNotFound. One outside the caller's scope stays readable
when a note in it mentions them.
*/
func visible(
	ctx context.Context, q *models.Queries, m authz.Membership, c models.Conversation,
) (models.Conversation, error) {
	if c.DeletedAt != nil {
		return c, exceptions.ErrNotFound
	}

	reads, err := authz.Reads(ctx, q, VisibleScope(m), c)
	if err != nil {
		return c, err
	}

	if !reads {
		return c, exceptions.ErrNotFound
	}

	return c, nil
}
