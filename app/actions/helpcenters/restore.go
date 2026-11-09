/*
|--------------------------------------------------------------------------
| Restoring a Version
|--------------------------------------------------------------------------
|
| Restoring a kept version puts its words, address and placing back into
| the working copy. Readers see nothing until it is published, which
| keeps a new version, so every restore is itself recorded and can be
| undone by restoring again.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Restore puts a version of the article back into its working copy and
returns the article as it now is.
*/
func (a *Articles) Restore(ctx context.Context, m authz.Membership, article, version ids.UUID,
) (models.Article, error) {
	if !m.Can("articles:write") {
		return models.Article{}, exceptions.ErrForbidden
	}

	now := a.Clock.Now()

	var out models.Article

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		kept, err := q.FindArticleVersion(ctx, models.FindArticleVersionParams{
			WorkspaceID: m.Workspace.ID, ID: version,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && kept.ArticleID != article) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("find version: %w", err)
		}

		out, err = a.save(ctx, q, restoredInput(m, article, kept), now)

		return err
	})

	return out, err
}

/*
restoredInput is the change that puts a kept version back.
*/
func restoredInput(m authz.Membership, article ids.UUID, kept models.ArticleVersion) ArticleInput {
	description := ""
	if kept.Description != nil {
		description = *kept.Description
	}

	return ArticleInput{
		Membership: m, ID: article, Title: &kept.Title, Slug: &kept.Slug, Description: &description,
		Body: &kept.BodyMarkdown, Visibility: &kept.Visibility, CollectionID: kept.CollectionID,
		ClearCollection: kept.CollectionID == nil,
	}
}
