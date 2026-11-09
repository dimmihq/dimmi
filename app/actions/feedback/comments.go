/*
|--------------------------------------------------------------------------
| Comments
|--------------------------------------------------------------------------
|
| Customers and the team talk under a request, one level of replies
| deep. The team can also leave internal notes, which only the team ever
| sees. Only public comments count toward a request's comments.
|
*/

package feedback

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of commenting.
*/
var (
	ErrCommentInvalid = fault.New(fault.Invalid, "comment_invalid",
		"Comments are 1 to 5,000 characters.").OnField("body")
	ErrCommentNotFound = fault.New(fault.NotFound, "feedback_comment_not_found",
		"No comment of this workspace has this id.")
)

/*
CommentInput is a comment to leave: Parent replies to another, and
Internal keeps it to the team.
*/
type CommentInput struct {
	Post     ids.UUID
	Parent   *ids.UUID
	Body     string
	Internal bool
}

/*
author is who leaves a comment: a teammate or a customer.
*/
type author struct {
	ws      ids.UUID
	member  *ids.UUID
	contact *ids.UUID
}

/*
Comment leaves a teammate's reply or internal note.
*/
func (p *Posts) Comment(ctx context.Context, m authz.Membership, in CommentInput) (models.FeedbackComment, error) {
	if !m.Can("feedback:write") {
		return models.FeedbackComment{}, exceptions.ErrForbidden
	}
	var out models.FeedbackComment
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = p.addComment(ctx, models.New(tx), author{ws: m.Workspace.ID, member: memberOf(m)}, in)
		return err
	})
	return out, err
}

func (p *Posts) addComment(ctx context.Context, q *models.Queries, by author, in CommentInput,
) (models.FeedbackComment, error) {
	body := strings.TrimSpace(in.Body)
	if body == "" || len([]rune(body)) > 5000 {
		return models.FeedbackComment{}, ErrCommentInvalid
	}
	post, err := lockPost(ctx, q, by.ws, in.Post)
	if err != nil {
		return models.FeedbackComment{}, err
	}
	if in.Parent != nil {
		parent, err := q.FindFeedbackComment(ctx, models.FindFeedbackCommentParams{WorkspaceID: by.ws, ID: *in.Parent})
		if err != nil || parent.PostID != post.ID {
			return models.FeedbackComment{}, missing(err, ErrCommentNotFound)
		}
		if parent.ParentID != nil {
			in.Parent = parent.ParentID
		}
	}
	out, err := q.CreateFeedbackComment(ctx, models.CreateFeedbackCommentParams{
		WorkspaceID: by.ws, PostID: post.ID, ParentID: in.Parent, AuthorContactID: by.contact,
		AuthorMemberID: by.member, Body: body, Internal: in.Internal && by.member != nil, Now: p.Clock.Now(),
	})
	if err != nil {
		return out, fmt.Errorf("comment on feature request: %w", err)
	}
	return out, recount(ctx, q, post)
}

/*
Comments returns a request's comments and internal notes, oldest first.
*/
func (p *Posts) Comments(ctx context.Context, m authz.Membership, post ids.UUID,
) ([]models.ListFeedbackCommentsRow, error) {
	if !m.Can("feedback:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.ListFeedbackCommentsRow
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findPost(ctx, q, m.Workspace.ID, post); err != nil {
			return err
		}
		var err error
		out, err = q.ListFeedbackComments(ctx, models.ListFeedbackCommentsParams{
			WorkspaceID: m.Workspace.ID, PostID: post, IncludeInternal: true,
		})
		return err
	})
	return out, err
}

/*
DeleteComment deletes a comment.
*/
func (p *Posts) DeleteComment(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("feedback:write") {
		return exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	return p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		c, err := q.FindFeedbackComment(ctx, models.FindFeedbackCommentParams{WorkspaceID: ws, ID: id})
		if err != nil {
			return missing(err, ErrCommentNotFound)
		}
		if err := q.DeleteFeedbackComment(ctx, models.DeleteFeedbackCommentParams{
			Now: new(p.Clock.Now()), WorkspaceID: ws, ID: id,
		}); err != nil {
			return fmt.Errorf("delete feature request comment: %w", err)
		}
		post, err := findPost(ctx, q, ws, c.PostID)
		if err != nil {
			return err
		}
		return recount(ctx, q, post)
	})
}
