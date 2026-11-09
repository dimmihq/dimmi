/*
|--------------------------------------------------------------------------
| Feedback Controller
|--------------------------------------------------------------------------
|
| A help center's feature requests: listed a numbered page at a time,
| posted by the team, edited, moved through statuses, approved, merged
| and deleted.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/feedback"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
feedbackPage is how many requests a page lists.
*/
const feedbackPage = 25

/*
FeedbackController serves feature requests.
*/
type FeedbackController struct {
	Posts *feedback.Posts
	IDs   *ids.Registry
}

/*
FeedbackPostsOutput is a page of requests with how many are in each
status.
*/
type FeedbackPostsOutput struct {
	Body struct {
		fhttp.Collection[resources.FeedbackPost]
		Counts map[string]int32 `json:"counts"`
	}
}

/*
FeedbackPostOutput is one request.
*/
type FeedbackPostOutput struct {
	Body fhttp.Resource[resources.FeedbackPost]
}

/*
Index lists a page of a help center's requests.
*/
func (c *FeedbackController) Index(ctx context.Context, in *requests.IndexFeedbackPostsRequest,
) (*FeedbackPostsOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	offset, _, err := numberedPage(ctx, feedbackPage)
	if err != nil {
		return nil, err
	}
	page, err := c.Posts.List(ctx, membership(ctx), feedback.PostFilter{
		HelpCenterID: hc, Status: in.Status, Moderation: in.Moderation, Query: in.Query, Sort: in.Sort,
		Limit: feedbackPage, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	out := &FeedbackPostsOutput{}
	out.Body.Collection = fhttp.NewCollection(pagination.Page[resources.FeedbackPost]{
		Items: resources.NewFeedbackPosts(page.Posts), Limit: feedbackPage, Total: &page.Total,
	})
	out.Body.Counts = page.ByStatus
	return out, nil
}

/*
Store posts a request from the team.
*/
func (c *FeedbackController) Store(ctx context.Context, in *requests.StoreFeedbackPostRequest,
) (*FeedbackPostOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	input := feedback.PostInput{
		Membership: membership(ctx), HelpCenterID: hc, Title: &in.Body.Title, Body: &in.Body.BodyMarkdown,
	}
	if in.Body.ContactID != "" {
		contact, err := c.decode(models.PrefixContact, in.Body.ContactID)
		if err != nil {
			return nil, err
		}
		input.Contact = &contact
	}
	p, err := c.Posts.Create(ctx, input)
	return postOutput(p, err)
}

/*
Show answers one request.
*/
func (c *FeedbackController) Show(ctx context.Context, in *requests.FeedbackPostRequest) (*FeedbackPostOutput, error) {
	id, err := c.decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	p, err := c.Posts.Show(ctx, membership(ctx), id)
	return postOutput(p, err)
}

/*
Update changes a request's words, status or approval, or merges it.
*/
func (c *FeedbackController) Update(ctx context.Context, in *requests.UpdateFeedbackPostRequest,
) (*FeedbackPostOutput, error) {
	id, err := c.decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	m, b := membership(ctx), in.Body
	switch {
	case b.MergeInto != nil:
		target, err := c.decode(models.PrefixFeedbackPost, *b.MergeInto)
		if err != nil {
			return nil, err
		}
		return postOutput(c.Posts.Merge(ctx, m, id, target))
	case b.Moderation != nil:
		return postOutput(c.Posts.Moderate(ctx, m, id, *b.Moderation))
	case b.Status != nil:
		return c.move(ctx, id, in)
	}
	return postOutput(c.Posts.Update(ctx, feedback.PostInput{
		Membership: m, ID: id, Title: b.Title, Body: b.BodyMarkdown,
	}))
}

func (c *FeedbackController) move(ctx context.Context, id ids.UUID, in *requests.UpdateFeedbackPostRequest,
) (*FeedbackPostOutput, error) {
	b := in.Body
	input := feedback.StatusInput{
		Membership: membership(ctx), Post: id, Status: *b.Status, Notify: b.Notify, Message: b.Message,
	}
	if b.ChangelogEntryID != nil && *b.ChangelogEntryID != "" {
		entry, err := c.decode(models.PrefixChangelogEntry, *b.ChangelogEntryID)
		if err != nil {
			return nil, err
		}
		input.Entry = &entry
	}
	return postOutput(c.Posts.SetStatus(ctx, input))
}

/*
Destroy deletes a request.
*/
func (c *FeedbackController) Destroy(ctx context.Context, in *requests.FeedbackPostRequest) (*struct{}, error) {
	id, err := c.decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Posts.Delete(ctx, membership(ctx), id)
}

func postOutput(p feedback.Post, err error) (*FeedbackPostOutput, error) {
	if err != nil {
		return nil, err
	}
	return &FeedbackPostOutput{Body: fhttp.NewResource(resources.NewFeedbackPost(p))}, nil
}

func (c *FeedbackController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
