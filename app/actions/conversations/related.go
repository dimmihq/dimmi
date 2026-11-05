/*
|--------------------------------------------------------------------------
| Related Rows
|--------------------------------------------------------------------------
|
| The rows a page of conversations points at, read with one query per
| table for the whole page and matched back to each conversation by id.
|
*/

package conversations

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
relatedRows is everything a page of conversations points at, by id.
*/
type relatedRows struct {
	contacts    map[ids.UUID]models.Contact
	inboxes     map[ids.UUID]models.Inbox
	connections map[ids.UUID]models.ChannelConnection
	members     map[ids.UUID]Member
	teams       map[ids.UUID]models.Team
	last        map[ids.UUID]models.Message
	tags        map[ids.UUID][]models.Tag
	unread      map[ids.UUID]bool
}

/*
pageIDs are the workspace of a page of conversations and the ids the
page points at.
*/
type pageIDs struct {
	workspace                                                     ids.UUID
	conversations, contacts, inboxes, connections, members, teams []ids.UUID
}

/*
collect lists the ids the conversations point at.
*/
func collect(convs []models.Conversation) pageIDs {
	p := pageIDs{workspace: convs[0].WorkspaceID}

	for _, c := range convs {
		p.conversations = append(p.conversations, c.ID)
		p.contacts = append(p.contacts, c.ContactID)
		p.inboxes = append(p.inboxes, c.InboxID)
		p.connections = append(p.connections, c.ChannelConnectionID)

		if c.AssigneeMemberID != nil {
			p.members = append(p.members, *c.AssigneeMemberID)
		}

		if c.AssigneeTeamID != nil {
			p.teams = append(p.teams, *c.AssigneeTeamID)
		}
	}

	return p
}

/*
related reads the rows the conversations point at.
*/
func related(
	ctx context.Context, q *models.Queries, memberID ids.UUID, convs []models.Conversation,
) (relatedRows, error) {
	ws, p := convs[0].WorkspaceID, collect(convs)
	r := relatedRows{}

	contacts, err := q.ContactsByIDs(ctx, models.ContactsByIDsParams{WorkspaceID: ws, Ids: p.contacts})
	if err != nil {
		return r, err
	}

	inboxes, err := q.InboxesByIDs(ctx, models.InboxesByIDsParams{WorkspaceID: ws, Ids: p.inboxes})
	if err != nil {
		return r, err
	}

	connections, err := q.ChannelConnectionsByIDs(ctx, models.ChannelConnectionsByIDsParams{
		WorkspaceID: ws, Ids: p.connections,
	})
	if err != nil {
		return r, err
	}

	r.contacts = byID(contacts, func(c models.Contact) ids.UUID { return c.ID })
	r.inboxes = byID(inboxes, func(i models.Inbox) ids.UUID { return i.ID })
	r.connections = byID(connections, func(c models.ChannelConnection) ids.UUID { return c.ID })

	if err := r.readActivity(ctx, q, memberID, p); err != nil {
		return r, err
	}

	if err := r.readTags(ctx, q, p); err != nil {
		return r, err
	}

	for _, m := range r.last {
		if m.AuthorType == string(enums.AuthorTypeMember) && m.AuthorID != nil {
			p.members = append(p.members, *m.AuthorID)
		}
	}

	err = r.readPeople(ctx, q, ws, p)

	return r, err
}

/*
readPeople reads the members and teams holding the conversations.
*/
func (r *relatedRows) readPeople(ctx context.Context, q *models.Queries, ws ids.UUID, p pageIDs) error {
	members, err := Members(ctx, q, ws, p.members)
	if err != nil {
		return err
	}

	teams, err := q.TeamsByIDs(ctx, models.TeamsByIDsParams{WorkspaceID: ws, Ids: p.teams})
	if err != nil {
		return err
	}

	r.members = members
	r.teams = byID(teams, func(t models.Team) ids.UUID { return t.ID })

	return nil
}

/*
readActivity reads each conversation's latest message and whether the
member has unread entries in it.
*/
func (r *relatedRows) readActivity(ctx context.Context, q *models.Queries, memberID ids.UUID, p pageIDs) error {
	ws, convs := p.workspace, p.conversations

	last, err := q.LatestMessagesOf(ctx, models.LatestMessagesOfParams{WorkspaceID: ws, Ids: convs})
	if err != nil {
		return err
	}

	unread, err := q.UnreadConversationIDs(ctx, models.UnreadConversationIDsParams{
		WorkspaceID: ws, Ids: convs, MemberID: memberID,
	})
	if err != nil {
		return err
	}

	r.last = byID(last, func(m models.Message) ids.UUID { return m.ConversationID })
	r.unread = map[ids.UUID]bool{}

	for _, id := range unread {
		r.unread[id] = true
	}

	return nil
}

/*
readTags reads the tags on each conversation.
*/
func (r *relatedRows) readTags(ctx context.Context, q *models.Queries, p pageIDs) error {
	rows, err := q.ListConversationTags(ctx, models.ListConversationTagsParams{
		WorkspaceID: p.workspace, ConversationIds: p.conversations,
	})
	if err != nil {
		return err
	}

	r.tags = map[ids.UUID][]models.Tag{}
	for _, row := range rows {
		r.tags[row.ConversationID] = append(r.tags[row.ConversationID], row.Tag)
	}

	return nil
}

/*
loaded puts the conversation together with its rows.
*/
func (r *relatedRows) loaded(c models.Conversation) Loaded {
	l := Loaded{
		Conversation: c, Contact: r.contacts[c.ContactID], Inbox: r.inboxes[c.InboxID],
		Connection: r.connections[c.ChannelConnectionID], Unread: r.unread[c.ID], Tags: r.tags[c.ID],
	}

	if c.AssigneeMemberID != nil {
		if m, ok := r.members[*c.AssigneeMemberID]; ok {
			l.Assignee = &m
		}
	}

	if c.AssigneeTeamID != nil {
		if t, ok := r.teams[*c.AssigneeTeamID]; ok {
			l.Team = &t
		}
	}

	if m, ok := r.last[c.ID]; ok {
		l.LastMessage = &m
		if author, ok := r.members[orNil(m.AuthorID)]; ok && m.AuthorType == string(enums.AuthorTypeMember) {
			l.LastAuthor = &author
		}
	}

	return l
}

/*
Members reads the members of the workspace with the ids, by id.
*/
func Members(ctx context.Context, q *models.Queries, ws ids.UUID, memberIDs []ids.UUID) (map[ids.UUID]Member, error) {
	out := map[ids.UUID]Member{}
	if len(memberIDs) == 0 {
		return out, nil
	}

	rows, err := q.MembersByIDs(ctx, models.MembersByIDsParams{WorkspaceID: ws, Ids: memberIDs})
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		out[row.WorkspaceMember.ID] = Member{Member: row.WorkspaceMember, User: row.User}
	}

	return out, nil
}

/*
byID indexes the rows by the key.
*/
func byID[T any](rows []T, key func(T) ids.UUID) map[ids.UUID]T {
	out := make(map[ids.UUID]T, len(rows))
	for _, row := range rows {
		out[key(row)] = row
	}

	return out
}
