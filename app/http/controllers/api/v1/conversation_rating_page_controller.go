/*
|--------------------------------------------------------------------------
| Conversation Rating Page Controller
|--------------------------------------------------------------------------
|
| GET /conversation-ratings/{token} reads the ask a rating email links
| to; PATCH keeps the customer's score and comment. Nobody signs in: the
| signed link is the proof, and it names only its own ask. An ask older
| than a week answers 410 to a change.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/ratings"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
CustomerRatingOutput is an ask as its customer sees it.
*/
type CustomerRatingOutput struct {
	Body fhttp.Resource[resources.CustomerRating]
}

/*
ConversationRatingPageController serves the rating page.
*/
type ConversationRatingPageController struct {
	Ratings *ratings.Ratings
}

/*
Show reads the ask.
*/
func (c *ConversationRatingPageController) Show(
	ctx context.Context, in *requests.ConversationRatingTokenRequest,
) (*CustomerRatingOutput, error) {
	r, err := c.Ratings.ByToken(ctx, in.Token)
	if err != nil {
		return nil, err
	}

	return &CustomerRatingOutput{Body: fhttp.NewResource(resources.NewCustomerRating(r))}, nil
}

/*
Update keeps the customer's answer.
*/
func (c *ConversationRatingPageController) Update(
	ctx context.Context, in *requests.UpdateConversationRatingByTokenRequest,
) (*CustomerRatingOutput, error) {
	r, err := c.Ratings.RateByToken(ctx, in.Token, ratings.Answer{Score: in.Body.Score, Comment: in.Body.Comment})
	if err != nil {
		return nil, err
	}

	return &CustomerRatingOutput{Body: fhttp.NewResource(resources.NewCustomerRating(r))}, nil
}
