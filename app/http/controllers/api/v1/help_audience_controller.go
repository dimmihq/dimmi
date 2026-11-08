/*
|--------------------------------------------------------------------------
| Article Audience Controller
|--------------------------------------------------------------------------
|
| PATCH /articles/{article}/audience and /collections/{collection}/audience
| keep an article or a collection to a saved audience, or open it to
| everyone with a null audience, and answer it as it now is.
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
HelpAudienceController keeps articles and collections to audiences.
*/
type HelpAudienceController struct {
	Articles    *helpcenters.Articles
	Collections *helpcenters.Collections
	IDs         *ids.Registry
}

/*
ArticleAudienceOutput is the article as it now is.
*/
type ArticleAudienceOutput struct {
	Body fhttp.Resource[resources.Article]
}

/*
CollectionAudienceOutput is the collection as it now is.
*/
type CollectionAudienceOutput struct {
	Body fhttp.Resource[resources.Collection]
}

/*
Article keeps an article to an audience.
*/
func (c *HelpAudienceController) Article(ctx context.Context, in *requests.ArticleAudienceRequest,
) (*ArticleAudienceOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixArticle), in.Article)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	audience, err := c.audience(in.Body.AudienceID)
	if err != nil {
		return nil, err
	}

	a, err := c.Articles.SetArticleAudience(ctx, membership(ctx), id, audience)
	if err != nil {
		return nil, err
	}

	return &ArticleAudienceOutput{Body: fhttp.NewResource(resources.NewArticle(a))}, nil
}

/*
Collection keeps a collection to an audience.
*/
func (c *HelpAudienceController) Collection(ctx context.Context, in *requests.CollectionAudienceRequest,
) (*CollectionAudienceOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixCollection), in.Collection)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	audience, err := c.audience(in.Body.AudienceID)
	if err != nil {
		return nil, err
	}

	col, err := c.Collections.SetCollectionAudience(ctx, membership(ctx), id, audience)
	if err != nil {
		return nil, err
	}

	return &CollectionAudienceOutput{Body: fhttp.NewResource(resources.NewCollection(col))}, nil
}

/*
audience decodes an audience id, nil for everyone.
*/
func (c *HelpAudienceController) audience(public *string) (*ids.UUID, error) {
	if public == nil || *public == "" {
		return nil, nil
	}

	id, err := c.IDs.Decode(string(models.PrefixAudience), *public)
	if err != nil {
		return nil, helpcenters.ErrAudienceUnfit
	}

	return &id, nil
}
