/*
|--------------------------------------------------------------------------
| Article Extras Controller
|--------------------------------------------------------------------------
|
| POST /articles/{article}/restorations puts a kept version back into the
| working copy, and POST /help-images keeps a picture to place in an
| article, answering with the address it is served at.
|
*/

package v1

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ArticleExtrasController restores versions and keeps article pictures.
*/
type ArticleExtrasController struct {
	Articles *helpcenters.Articles
	Images   *helpcenters.Images
	IDs      *ids.Registry
}

/*
HelpImageOutput is where a kept picture is served.
*/
type HelpImageOutput struct {
	Body struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
}

/*
Restore puts the version back and answers with the article.
*/
func (c *ArticleExtrasController) Restore(ctx context.Context, in *requests.RestoreArticleRequest,
) (*ArticleOutput, error) {
	article, err := c.IDs.Decode(string(models.PrefixArticle), in.Article)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	version, err := c.IDs.Decode(string(models.PrefixArticleVersion), in.Body.VersionID)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	a, err := c.Articles.Restore(ctx, membership(ctx), article, version)

	return articleOutput(a, err)
}

/*
StoreImage keeps the uploaded picture.
*/
func (c *ArticleExtrasController) StoreImage(ctx context.Context, in *requests.StoreHelpImageRequest,
) (*HelpImageOutput, error) {
	defer func() { _ = in.RawBody.RemoveAll() }()

	uploads := in.RawBody.File["image"]
	if len(uploads) == 0 {
		return nil, helpcenters.ErrInvalidImage
	}

	file, err := uploads[0].Open()
	if err != nil {
		return nil, fmt.Errorf("open uploaded image: %w", err)
	}

	defer func() { _ = file.Close() }()

	url, err := c.Images.Store(ctx, membership(ctx), file)
	if err != nil {
		return nil, err
	}

	out := &HelpImageOutput{}
	out.Body.Data.URL = url

	return out, nil
}
