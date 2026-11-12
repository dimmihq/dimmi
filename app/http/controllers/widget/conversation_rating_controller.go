/*
|--------------------------------------------------------------------------
| Widget Conversation Rating Controller
|--------------------------------------------------------------------------
|
| A visitor rates a conversation that closed and asked them how it
| went. The messenger reads the latest ask of a conversation to draw the
| rating card, and sends the face the visitor picked and any comment.
| The conversation is looked up among the visitor's own first, so an id
| of anyone else's is simply not found.
|
*/

package widget

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ratings"
	actions "github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RatingOutput is the conversation's latest ask, null when none went out.
*/
type RatingOutput struct {
	Body struct {
		Data *resources.CustomerRating `json:"data"`
	}
}

/*
RatingController serves the rating card of a visitor's conversation.
*/
type RatingController struct {
	Read    *actions.ListWidgetConversations
	Ratings *ratings.Ratings
	IDs     *ids.Registry
}

/*
Show answers with the conversation's latest ask.
*/
func (c *RatingController) Show(
	ctx context.Context, in *requests.ShowWidgetConversationRequest,
) (*RatingOutput, error) {
	found, err := c.visible(ctx, in.Conversation)
	if err != nil {
		return nil, err
	}

	r, err := c.Ratings.ForConversation(ctx, found.WorkspaceID, found.ID)
	if err != nil {
		return nil, err
	}

	out := &RatingOutput{}
	if r != nil {
		shaped := resources.NewCustomerRating(*r)
		out.Body.Data = &shaped
	}

	return out, nil
}

/*
Update keeps the visitor's answer.
*/
func (c *RatingController) Update(
	ctx context.Context, in *requests.UpdateWidgetConversationRatingRequest,
) (*RatingOutput, error) {
	found, err := c.visible(ctx, in.Conversation)
	if err != nil {
		return nil, err
	}

	r, err := c.Ratings.RateConversation(ctx, found.WorkspaceID, found.ID,
		ratings.Answer{Score: in.Body.Score, Comment: in.Body.Comment})
	if err != nil {
		return nil, err
	}

	shaped := resources.NewCustomerRating(r)
	out := &RatingOutput{}
	out.Body.Data = &shaped

	return out, nil
}

/*
visible returns the visitor's conversation the id names.
*/
func (c *RatingController) visible(ctx context.Context, public string) (models.Conversation, error) {
	id, err := c.IDs.Decode(string(models.PrefixConversation), public)
	if err != nil {
		return models.Conversation{}, exceptions.ErrWidgetConversationNotFound
	}

	found, err := c.Read.Find(ctx, authenticated(ctx), id)
	if err != nil {
		return models.Conversation{}, err
	}

	return found.Conversation, nil
}
