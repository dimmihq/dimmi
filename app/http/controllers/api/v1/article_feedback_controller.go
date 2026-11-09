/*
|--------------------------------------------------------------------------
| Article Feedback Controller
|--------------------------------------------------------------------------
|
| What readers said about an article, for the team beside the article:
| how many found it helpful, and the notes left by those who did not.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ArticleFeedbackController answers what readers said about an article.
*/
type ArticleFeedbackController struct {
	Ratings *helpcenters.Ratings
	IDs     *ids.Registry
}

/*
ArticleFeedbackOutput is what readers said about one article.
*/
type ArticleFeedbackOutput struct {
	Body fhttp.Resource[resources.ArticleFeedback]
}

/*
Show answers the article's counts and latest notes.
*/
func (c *ArticleFeedbackController) Show(ctx context.Context, in *requests.ArticleRequest,
) (*ArticleFeedbackOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixArticle), in.Article)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}
	f, err := c.Ratings.For(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	return &ArticleFeedbackOutput{Body: fhttp.NewResource(resources.NewArticleFeedback(f))}, nil
}
