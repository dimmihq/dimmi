/*
|--------------------------------------------------------------------------
| Requests From Conversations
|--------------------------------------------------------------------------
|
| A customer asks for something in a conversation; a teammate links the
| conversation to the request it asks for, found by its title or posted
| right there from the conversation. Linking votes for the customer on
| the team's behalf, once, so what support hears counts, and the
| request lists the conversations and customers behind it. When it
| moves, telling the people behind it reaches those customers too.
| Only someone who can see the conversation may link it.
|
*/

package feedback

import (
	"cmp"
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrLinkTargetMissing refuses a link that names neither a request nor a
title for a new one.
*/
var ErrLinkTargetMissing = fault.New(fault.Invalid, "post_id_required",
	"Pick a request to link, or give a title to post a new one.").OnField("post_id")

/*
LinkInput links a conversation to a request: the request Post names, or
a new one posted from the conversation on HelpCenterID's Board with
Title and Body. Priority is how much it matters to the customer.
*/
type LinkInput struct {
	Membership   authz.Membership
	Conversation ids.UUID
	Post         *ids.UUID
	HelpCenterID *ids.UUID
	Board        *ids.UUID
	Title        *string
	Body         *string
	Priority     string
}

/*
Link links the conversation to the request and votes for its customer,
returning the request. Linking it again changes nothing.
*/
func (p *Posts) Link(ctx context.Context, in LinkInput) (Post, error) {
	m := in.Membership
	if !m.Can("feedback:write") || !m.Can("conversations:read") {
		return Post{}, exceptions.ErrForbidden
	}
	if in.Post == nil && in.Title == nil {
		return Post{}, ErrLinkTargetMissing
	}
	priority, err := priorityOf(in.Priority)
	if err != nil {
		return Post{}, err
	}
	ws := m.Workspace.ID
	var out Post
	err = p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := conversations.Find(ctx, q, m, conversations.Ref{ID: in.Conversation})
		if err != nil {
			return err
		}
		row, err := p.target(ctx, q, in)
		if err != nil {
			return err
		}
		if err := p.posted(ctx, tx, in, row); err != nil {
			return err
		}
		if err := p.attach(ctx, tx, c, vote{post: row, member: memberOf(m), priority: priority}); err != nil {
			return err
		}
		out, err = p.reload(ctx, q, ws, row.ID)
		return err
	})
	return out, err
}

/*
posted announces the request a link just posted, when it named none.
*/
func (p *Posts) posted(ctx context.Context, tx *database.Tx, in LinkInput, row models.FeedbackPost) error {
	if in.Post != nil {
		return nil
	}
	m := in.Membership
	base := p.byMember(m.Workspace.ID, m.Member.ID)
	return p.announce(ctx, tx, appevents.FeedbackPostCreated{Base: base, PostID: row.ID})
}

/*
attach links the conversation to the request for the teammate the vote
names and, the first time, counts its customer's vote with the vote's
priority.
*/
func (p *Posts) attach(ctx context.Context, tx *database.Tx, c models.Conversation, v vote) error {
	q := models.New(tx)
	v.contact, v.now = c.ContactID, p.Clock.Now()
	linked, err := q.LinkFeedbackConversation(ctx, models.LinkFeedbackConversationParams{
		PostID: v.post.ID, ConversationID: c.ID, WorkspaceID: c.WorkspaceID, ContactID: &c.ContactID,
		MemberID: v.member, Now: v.now,
	})
	if err != nil {
		return fmt.Errorf("link conversation to feature request: %w", err)
	}
	if linked == 0 {
		return nil
	}
	if err := castVote(ctx, q, v); err != nil {
		return err
	}
	member := *cmp.Or(v.member, new(ids.Nil()))
	if err := p.announce(ctx, tx, appevents.FeedbackPostLinked{
		Base: p.byMember(c.WorkspaceID, member), PostID: v.post.ID, ConversationID: c.ID,
	}); err != nil {
		return err
	}
	return p.voted(ctx, tx, p.byMember(c.WorkspaceID, member), v)
}

/*
target returns the request to link: the one named, or a new one posted
for nobody yet, which the link's vote then counts the customer on.
*/
func (p *Posts) target(ctx context.Context, q *models.Queries, in LinkInput) (models.FeedbackPost, error) {
	ws := in.Membership.Workspace.ID
	if in.Post != nil {
		return lockPost(ctx, q, ws, *in.Post)
	}
	title, body, err := checked(in.Title, in.Body)
	if err != nil {
		return models.FeedbackPost{}, err
	}
	center, err := centerFor(ctx, q, ws, in.HelpCenterID)
	if err != nil {
		return models.FeedbackPost{}, err
	}
	board, err := boardOf(ctx, q, center, in.Board)
	if err != nil {
		return models.FeedbackPost{}, err
	}
	return p.insert(ctx, q, draft{
		ws: ws, hc: center.ID, board: board.ID, title: title, body: body, moderation: "approved",
		member: memberOf(in.Membership),
	})
}

/*
centerFor returns the help center named, or the workspace's default.
*/
func centerFor(ctx context.Context, q *models.Queries, ws ids.UUID, id *ids.UUID) (models.HelpCenter, error) {
	if id != nil {
		h, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{WorkspaceID: ws, ID: *id})
		return h, missing(err, helpcenters.ErrHelpCenterNotFound)
	}
	h, err := q.FindDefaultHelpCenter(ctx, ws)
	return h, missing(err, helpcenters.ErrHelpCenterNotFound)
}

/*
Unlink removes a conversation's link to a request. The customer's vote
stays; the team takes it back from the request's voters.
*/
func (p *Posts) Unlink(ctx context.Context, m authz.Membership, conversation, post ids.UUID) error {
	if !m.Can("feedback:write") || !m.Can("conversations:read") {
		return exceptions.ErrForbidden
	}
	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := conversations.Find(ctx, q, m, conversations.Ref{ID: conversation}); err != nil {
			return err
		}
		removed, err := q.UnlinkFeedbackConversation(ctx, models.UnlinkFeedbackConversationParams{
			WorkspaceID: m.Workspace.ID, PostID: post, ConversationID: conversation,
		})
		if err != nil {
			return fmt.Errorf("unlink conversation: %w", err)
		}
		if removed == 0 {
			return ErrPostNotFound
		}
		return nil
	})
}

/*
ConversationPosts returns the requests linked to a conversation.
*/
func (p *Posts) ConversationPosts(ctx context.Context, m authz.Membership, conversation ids.UUID) ([]Post, error) {
	if !m.Can("feedback:read") || !m.Can("conversations:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []Post
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := conversations.Find(ctx, q, m, conversations.Ref{ID: conversation})
		if err != nil {
			return err
		}
		rows, err := q.ListConversationFeedbackPosts(ctx, models.ListConversationFeedbackPostsParams{
			WorkspaceID: m.Workspace.ID, ConversationID: c.ID,
		})
		if err != nil {
			return fmt.Errorf("list linked feature requests: %w", err)
		}
		out, err = named(ctx, q, m.Workspace.ID, rows)
		return err
	})
	return out, err
}

/*
PostConversations returns the conversations linked to a request, with
their customers.
*/
func (p *Posts) PostConversations(ctx context.Context, m authz.Membership, post ids.UUID,
) ([]models.ListFeedbackPostConversationsRow, error) {
	if !m.Can("feedback:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.ListFeedbackPostConversationsRow
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findPost(ctx, q, m.Workspace.ID, post); err != nil {
			return err
		}
		var err error
		access := m.ConversationScope().Params()
		out, err = q.ListFeedbackPostConversations(ctx, models.ListFeedbackPostConversationsParams{
			WorkspaceID: m.Workspace.ID, PostID: post, AccessMode: access.Mode, AccessViewer: access.Viewer,
			AccessTeams: access.Teams, AccessUnassigned: access.Unassigned, AccessHiddenInboxes: access.HiddenInboxes,
		})
		return err
	})
	return out, err
}
