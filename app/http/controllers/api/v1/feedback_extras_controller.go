/*
|--------------------------------------------------------------------------
| Feedback Votes and Comments Controller
|--------------------------------------------------------------------------
|
| Who voted for a feature request, votes the team casts for customers,
| and the team's comments and internal notes on it.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
FeedbackExtrasController serves a request's votes and comments.
*/
type FeedbackExtrasController struct {
	Posts *feedback.Posts
	IDs   *ids.Registry
}

/*
FeedbackVotersOutput is a request's voters.
*/
type FeedbackVotersOutput struct {
	Body fhttp.Collection[resources.FeedbackVoter]
}

/*
FeedbackCommentsOutput is a request's comments and internal notes.
*/
type FeedbackCommentsOutput struct {
	Body fhttp.Collection[resources.FeedbackComment]
}

/*
FeedbackCommentOutput is one comment.
*/
type FeedbackCommentOutput struct {
	Body fhttp.Resource[resources.FeedbackComment]
}

/*
Voters lists who voted for a request.
*/
func (c *FeedbackExtrasController) Voters(ctx context.Context, in *requests.FeedbackPostRequest,
) (*FeedbackVotersOutput, error) {
	id, err := c.decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	rows, err := c.Posts.Voters(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.FeedbackVoter]{Items: resources.NewFeedbackVoters(rows), Limit: 500}
	return &FeedbackVotersOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Vote counts a vote the team casts for a customer.
*/
func (c *FeedbackExtrasController) Vote(ctx context.Context, in *requests.StoreFeedbackVoteRequest,
) (*FeedbackPostOutput, error) {
	id, err := c.decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	contact, err := c.decode(models.PrefixContact, in.Body.ContactID)
	if err != nil {
		return nil, err
	}
	return postOutput(c.Posts.VoteFor(ctx, feedback.VoteInput{
		Membership: membership(ctx), Post: id, Contact: contact, Priority: in.Body.Priority,
	}))
}

/*
Unvote takes a customer's vote back.
*/
func (c *FeedbackExtrasController) Unvote(ctx context.Context, in *requests.DestroyFeedbackVoteRequest,
) (*FeedbackPostOutput, error) {
	id, err := c.decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	contact, err := c.decode(models.PrefixContact, in.Contact)
	if err != nil {
		return nil, err
	}
	return postOutput(c.Posts.Unvote(ctx, membership(ctx), id, contact))
}

/*
Comments lists a request's comments and internal notes.
*/
func (c *FeedbackExtrasController) Comments(ctx context.Context, in *requests.FeedbackPostRequest,
) (*FeedbackCommentsOutput, error) {
	id, err := c.decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	rows, err := c.Posts.Comments(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.FeedbackComment]{Items: resources.NewFeedbackComments(rows), Limit: 1000}
	return &FeedbackCommentsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
StoreComment leaves the team's reply or internal note.
*/
func (c *FeedbackExtrasController) StoreComment(ctx context.Context, in *requests.StoreFeedbackCommentRequest,
) (*FeedbackCommentOutput, error) {
	id, err := c.decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	input := feedback.CommentInput{Post: id, Body: in.Body.Body, Internal: in.Body.Internal}
	if in.Body.ParentID != "" {
		parent, err := c.decode(models.PrefixFeedbackComment, in.Body.ParentID)
		if err != nil {
			return nil, err
		}
		input.Parent = &parent
	}
	row, err := c.Posts.Comment(ctx, membership(ctx), input)
	if err != nil {
		return nil, err
	}
	all, err := c.Posts.Comments(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	for _, shaped := range resources.NewFeedbackComments(all) {
		if shaped.ID == models.PublicID(models.PrefixFeedbackComment, row.ID) {
			return &FeedbackCommentOutput{Body: fhttp.NewResource(shaped)}, nil
		}
	}
	return nil, feedback.ErrCommentNotFound
}

/*
DestroyComment deletes a comment.
*/
func (c *FeedbackExtrasController) DestroyComment(ctx context.Context, in *requests.FeedbackCommentRequest,
) (*struct{}, error) {
	id, err := c.decode(models.PrefixFeedbackComment, in.Comment)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Posts.DeleteComment(ctx, membership(ctx), id)
}

func (c *FeedbackExtrasController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	return (&FeedbackController{IDs: c.IDs}).decode(prefix, public)
}
