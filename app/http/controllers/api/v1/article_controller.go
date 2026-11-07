/*
|--------------------------------------------------------------------------
| Article Controller
|--------------------------------------------------------------------------
|
| A help center's articles: listed, created, saved, published,
| unpublished, archived, trashed, and their version history.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ArticleController serves articles.
*/
type ArticleController struct {
	Articles *helpcenters.Articles
	IDs      *ids.Registry
}

/*
ArticlesOutput is a list of articles.
*/
type ArticlesOutput struct {
	Body fhttp.Collection[resources.Article]
}

/*
ArticleOutput is one article with its body.
*/
type ArticleOutput struct {
	Body fhttp.Resource[resources.Article]
}

/*
ArticleVersionsOutput is an article's versions.
*/
type ArticleVersionsOutput struct {
	Body fhttp.Collection[resources.ArticleVersion]
}

/*
Index lists a help center's articles.
*/
func (c *ArticleController) Index(ctx context.Context, in *requests.IndexArticlesRequest) (*ArticlesOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	filter := helpcenters.ArticleFilter{HelpCenterID: hc, Status: in.Status, Query: in.Query, Limit: 200}
	if in.Collection != "" {
		col, err := c.decode(models.PrefixCollection, in.Collection)
		if err != nil {
			return nil, err
		}
		filter.CollectionID = &col
	}
	rows, err := c.Articles.List(ctx, membership(ctx), filter)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.Article]{Items: resources.NewArticles(rows), Limit: 200}
	return &ArticlesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store creates a draft.
*/
func (c *ArticleController) Store(ctx context.Context, in *requests.StoreArticleRequest) (*ArticleOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	input, err := c.input(ctx, in.Body)
	if err != nil {
		return nil, err
	}
	input.HelpCenterID = hc
	a, err := c.Articles.Create(ctx, input)
	return articleOutput(a, err)
}

/*
Show answers one article with its body.
*/
func (c *ArticleController) Show(ctx context.Context, in *requests.ArticleRequest) (*ArticleOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	a, err := c.Articles.Show(ctx, membership(ctx), id)
	return articleOutput(a, err)
}

/*
Update saves the working copy, or moves the article to draft or the
archive.
*/
func (c *ArticleController) Update(ctx context.Context, in *requests.UpdateArticleRequest) (*ArticleOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	if in.Body.Status != nil {
		a, err := c.Articles.SetStatus(ctx, membership(ctx), id, *in.Body.Status)
		return articleOutput(a, err)
	}
	input, err := c.input(ctx, in.Body.ArticleBody)
	if err != nil {
		return nil, err
	}
	input.ID, input.LockVersion = id, in.Body.LockVersion
	a, err := c.Articles.Update(ctx, input)
	return articleOutput(a, err)
}

/*
Publish makes the working copy live.
*/
func (c *ArticleController) Publish(ctx context.Context, in *requests.ArticleRequest) (*ArticleOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	a, err := c.Articles.Publish(ctx, membership(ctx), id)
	return articleOutput(a, err)
}

/*
Destroy moves an article to the trash.
*/
func (c *ArticleController) Destroy(ctx context.Context, in *requests.ArticleRequest) (*struct{}, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Articles.Delete(ctx, membership(ctx), id)
}

/*
Versions lists an article's versions.
*/
func (c *ArticleController) Versions(ctx context.Context, in *requests.ArticleRequest,
) (*ArticleVersionsOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	rows, err := c.Articles.Versions(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.ArticleVersion]{Items: resources.NewArticleVersions(rows), Limit: 1000}
	return &ArticleVersionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
input reads an article body; an empty collection_id uncollects it.
*/
func (c *ArticleController) input(ctx context.Context, b requests.ArticleBody) (helpcenters.ArticleInput, error) {
	out := helpcenters.ArticleInput{
		Membership: membership(ctx), Title: b.Title, Slug: b.Slug, Description: b.Description, Body: b.BodyMarkdown,
		Visibility: b.Visibility,
	}
	if b.CollectionID == nil {
		return out, nil
	}
	if *b.CollectionID == "" {
		out.ClearCollection = true
		return out, nil
	}
	col, err := c.decode(models.PrefixCollection, *b.CollectionID)
	if err != nil {
		return out, helpcenters.ErrCollectionNotFound
	}
	out.CollectionID = &col
	return out, nil
}

func articleOutput(a models.Article, err error) (*ArticleOutput, error) {
	if err != nil {
		return nil, err
	}
	return &ArticleOutput{Body: fhttp.NewResource(resources.NewArticle(a))}, nil
}

func (c *ArticleController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
