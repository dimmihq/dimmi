/*
|--------------------------------------------------------------------------
| Loaded Conversations
|--------------------------------------------------------------------------
|
| A conversation is never shown alone: its customer, inbox, channel, who
| holds it, its latest message and whether the reader has seen it all
| come along. They are loaded a page at a time, one query per kind of
| row whatever the page size, so a long list costs the same few queries
| as a short one.
|
| Which conversations a member may see is decided in one place. Today
| every member of a workspace sees all of its conversations; anything
| outside what a member may see is simply not found.
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
Ref names a conversation by its id, or by its number when the id is
nil.
*/
type Ref struct {
	ID     ids.UUID
	Number int64
}

/*
Member is a member of the workspace with their account.
*/
type Member struct {
	Member models.WorkspaceMember
	User   models.User
}

/*
Loaded is a conversation with everything shown beside it. Assignee,
Team and LastMessage are nil when there is none, and LastAuthor when a
member did not write the latest message; Unread is judged for the
member who loaded it.
*/
type Loaded struct {
	Conversation models.Conversation
	Contact      models.Contact
	Inbox        models.Inbox
	Connection   models.ChannelConnection
	Assignee     *Member
	Team         *models.Team
	LastMessage  *models.Message
	LastAuthor   *Member
	Unread       bool
}

/*
CanRead reports whether the caller may read the workspace's
conversations at all, refusing them otherwise.
*/
func CanRead(m authz.Membership) error {
	if !m.Can("conversations:read") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
CanWrite reports whether the caller may work the workspace's
conversations, refusing them otherwise.
*/
func CanWrite(m authz.Membership) error {
	if !m.Can("conversations:write") {
		return exceptions.ErrForbidden
	}

	return nil
}

/*
Find returns the live conversation the reference names in the
workspace, or exceptions.ErrNotFound.
*/
func Find(ctx context.Context, q *models.Queries, workspaceID ids.UUID, ref Ref) (models.Conversation, error) {
	var (
		c   models.Conversation
		err error
	)

	if ref.ID == ids.Nil() {
		c, err = q.FindConversationByNumber(ctx, models.FindConversationByNumberParams{
			WorkspaceID: workspaceID, Number: ref.Number,
		})
	} else {
		c, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: workspaceID, ID: ref.ID})
	}

	return visible(c, err)
}

/*
Lock returns the live conversation the reference names and holds its
row until the transaction ends.
*/
func Lock(ctx context.Context, q *models.Queries, workspaceID ids.UUID, ref Ref) (models.Conversation, error) {
	found, err := Find(ctx, q, workspaceID, ref)
	if err != nil {
		return found, err
	}

	locked, err := q.LockConversation(ctx, models.LockConversationParams{WorkspaceID: workspaceID, ID: found.ID})

	return visible(locked, err)
}

/*
VisibleScope is the rule the inbox shows conversations by, for the
realtime channels: a member who may read conversations sees every live
one of the workspace, and none in the trash, as lists and search do.
*/
type VisibleScope struct{}

/*
Contains reports whether the member may see the conversation.
*/
func (VisibleScope) Contains(_ context.Context, m authz.Membership, c models.Conversation) bool {
	return CanRead(m) == nil && c.DeletedAt == nil
}

/*
visible turns a missing or deleted conversation into
exceptions.ErrNotFound.
*/
func visible(c models.Conversation, err error) (models.Conversation, error) {
	if errors.Is(err, database.ErrNotFound) || (err == nil && c.DeletedAt != nil) {
		return c, exceptions.ErrNotFound
	}

	if err != nil {
		return c, fmt.Errorf("find conversation: %w", err)
	}

	return c, nil
}

/*
LoadOne loads one conversation for the member.
*/
func LoadOne(ctx context.Context, q *models.Queries, memberID ids.UUID, c models.Conversation) (Loaded, error) {
	loaded, err := Load(ctx, q, memberID, []models.Conversation{c})
	if err != nil {
		return Loaded{}, err
	}

	return loaded[0], nil
}

/*
Load loads the conversations of one workspace for the member, in the
order given.
*/
func Load(ctx context.Context, q *models.Queries, memberID ids.UUID, convs []models.Conversation) ([]Loaded, error) {
	if len(convs) == 0 {
		return nil, nil
	}

	r, err := related(ctx, q, memberID, convs)
	if err != nil {
		return nil, fmt.Errorf("load conversations: %w", err)
	}

	out := make([]Loaded, len(convs))
	for i, c := range convs {
		out[i] = r.loaded(c)
	}

	return out, nil
}
