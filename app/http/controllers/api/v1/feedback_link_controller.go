/*
|--------------------------------------------------------------------------
| Feedback Link Controller
|--------------------------------------------------------------------------
|
| Feature requests and the conversations they came up in: the requests
| a conversation is linked to, linking one from the inbox, found or
| posted from the conversation, taking a link back, and the
| conversations and customers behind a request.
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
FeedbackLinkController serves the links between requests and
conversations.
*/
type FeedbackLinkController struct {
	Posts *feedback.Posts
	IDs   *ids.Registry
}

/*
FeedbackPostListOutput is the requests linked to a conversation.
*/
type FeedbackPostListOutput struct {
	Body fhttp.Collection[resources.FeedbackPost]
}

/*
FeedbackConversationsOutput is the conversations linked to a request.
*/
type FeedbackConversationsOutput struct {
	Body fhttp.Collection[resources.FeedbackConversation]
}

/*
Index lists the requests linked to a conversation.
*/
func (c *FeedbackLinkController) Index(ctx context.Context, in *requests.ConversationFeedbackRequest,
) (*FeedbackPostListOutput, error) {
	conv, err := c.ids().decode(models.PrefixConversation, in.Conversation)
	if err != nil {
		return nil, err
	}
	posts, err := c.Posts.ConversationPosts(ctx, membership(ctx), conv)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.FeedbackPost]{Items: resources.NewFeedbackPosts(posts), Limit: 50}
	return &FeedbackPostListOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store links a conversation to a request, posting it first when asked,
and votes for the conversation's customer.
*/
func (c *FeedbackLinkController) Store(ctx context.Context, in *requests.LinkFeedbackPostRequest,
) (*FeedbackPostOutput, error) {
	d, b := c.ids(), in.Body
	conv, err := d.decode(models.PrefixConversation, in.Conversation)
	if err != nil {
		return nil, err
	}
	input := feedback.LinkInput{
		Membership: membership(ctx), Conversation: conv, Title: b.Title, Body: b.BodyMarkdown, Priority: b.Priority,
	}
	if input.Post, err = d.optional(models.PrefixFeedbackPost, b.PostID); err != nil {
		return nil, err
	}
	if input.HelpCenterID, err = d.optional(models.PrefixHelpCenter, b.HelpCenterID); err != nil {
		return nil, err
	}
	if input.Board, err = d.optional(models.PrefixFeedbackBoard, b.BoardID); err != nil {
		return nil, err
	}
	return postOutput(c.Posts.Link(ctx, input))
}

/*
Destroy takes a conversation's link to a request back.
*/
func (c *FeedbackLinkController) Destroy(ctx context.Context, in *requests.UnlinkFeedbackPostRequest,
) (*struct{}, error) {
	conv, err := c.ids().decode(models.PrefixConversation, in.Conversation)
	if err != nil {
		return nil, err
	}
	post, err := c.ids().decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Posts.Unlink(ctx, membership(ctx), conv, post)
}

/*
Conversations lists the conversations linked to a request.
*/
func (c *FeedbackLinkController) Conversations(ctx context.Context, in *requests.FeedbackPostRequest,
) (*FeedbackConversationsOutput, error) {
	post, err := c.ids().decode(models.PrefixFeedbackPost, in.Post)
	if err != nil {
		return nil, err
	}
	rows, err := c.Posts.PostConversations(ctx, membership(ctx), post)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.FeedbackConversation]{Items: resources.NewFeedbackConversations(rows), Limit: 200}
	return &FeedbackConversationsOutput{Body: fhttp.NewCollection(page)}, nil
}

func (c *FeedbackLinkController) ids() *FeedbackController {
	return &FeedbackController{Posts: c.Posts, IDs: c.IDs}
}
