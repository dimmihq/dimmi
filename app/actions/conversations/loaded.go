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
*/

package conversations

import (
	"context"
	"fmt"

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
	Tags         []models.Tag
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
