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

	return visible(m, c, err)
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

	return visible(m, locked, err)
}

/*
visible turns a missing, deleted or hidden conversation into
exceptions.ErrNotFound.
*/
func visible(m authz.Membership, c models.Conversation, err error) (models.Conversation, error) {
	if errors.Is(err, database.ErrNotFound) {
		return c, exceptions.ErrNotFound
	}

	if err != nil {
		return c, fmt.Errorf("find conversation: %w", err)
	}

	if c.DeletedAt != nil || !VisibleScope(m).Contains(c) {
		return c, exceptions.ErrNotFound
	}

	return c, nil
}
