/*
|--------------------------------------------------------------------------
| Conversation Rating Controller
|--------------------------------------------------------------------------
|
| GET and PATCH /conversation-rating-settings read and change whether
| and how the workspace asks customers to rate closed conversations.
| GET /conversations/{conversation}/ratings lists how the customer rated
| the conversation, newest first.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ratings"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
ConversationRatingSettingsOutput is the workspace's rating settings.
*/
type ConversationRatingSettingsOutput struct {
	Body fhttp.Resource[resources.ConversationRatingSettings]
}

/*
ConversationRatingsOutput is a conversation's ratings.
*/
type ConversationRatingsOutput struct {
	Body struct {
		Data []resources.ConversationRating `json:"data"`
	}
}

/*
ConversationRatingController serves the rating settings and a
conversation's ratings.
*/
type ConversationRatingController struct {
	Ratings *ratings.Ratings
	IDs     *ids.Registry
}

/*
Settings answers with the workspace's rating settings.
*/
func (c *ConversationRatingController) Settings(
	ctx context.Context, _ *requests.NoInput,
) (*ConversationRatingSettingsOutput, error) {
	s, err := c.Ratings.SettingsOf(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	return &ConversationRatingSettingsOutput{Body: fhttp.NewResource(resources.NewConversationRatingSettings(s))}, nil
}

/*
UpdateSettings changes the fields the request names.
*/
func (c *ConversationRatingController) UpdateSettings(
	ctx context.Context, in *requests.UpdateConversationRatingSettingsRequest,
) (*ConversationRatingSettingsOutput, error) {
	m := membership(ctx)

	s, err := c.Ratings.SettingsOf(ctx, m)
	if err != nil {
		return nil, err
	}

	b := in.Body
	keepGiven(&s.Enabled, b.Enabled)
	keepGiven(&s.DelayMinutes, b.DelayMinutes)
	keepGiven(&s.IncludeAgentClosed, b.IncludeAgentClosed)
	keepGiven(&s.Messenger, b.Messenger)
	keepGiven(&s.Email, b.Email)

	saved, err := c.Ratings.SaveSettings(ctx, m, s)
	if err != nil {
		return nil, err
	}

	body := fhttp.NewResource(resources.NewConversationRatingSettings(saved))

	return &ConversationRatingSettingsOutput{Body: body}, nil
}

/*
Index lists the conversation's ratings.
*/
func (c *ConversationRatingController) Index(
	ctx context.Context, in *requests.ConversationRequest,
) (*ConversationRatingsOutput, error) {
	ref, err := conversationRef(c.IDs, in.Conversation)
	if err != nil {
		return nil, err
	}

	rows, err := c.Ratings.OfConversation(ctx, membership(ctx), ref)
	if err != nil {
		return nil, err
	}

	out := &ConversationRatingsOutput{}
	out.Body.Data = resources.NewConversationRatings(rows)

	return out, nil
}

/*
keepGiven copies a value the request gave over the current one.
*/
func keepGiven[T any](current, given *T) {
	if given != nil {
		*current = *given
	}
}
