/*
|--------------------------------------------------------------------------
| Publishing Helpers
|--------------------------------------------------------------------------
|
| What every change readers can see shares: the help center's cache
| version moves on, and an address that changed keeps answering with a
| redirect to the new one.
|
*/

package helpcenters

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
place names a help center inside its workspace.
*/
type place struct {
	workspace  ids.UUID
	helpCenter ids.UUID
}

/*
target is where a redirect leads: an article or a collection.
*/
type target struct {
	article    *ids.UUID
	collection *ids.UUID
}

/*
move is an address that changed: the old path, where it now leads and
when it moved.
*/
type move struct {
	from string
	to   target
	now  time.Time
}

/*
redirect makes an old path answer with the target.
*/
func redirect(ctx context.Context, q *models.Queries, at place, m move) error {
	from, to, now := m.from, m.to, m.now
	kind := "article"
	if to.collection != nil {
		kind = "collection"
	}
	err := q.UpsertRedirect(ctx, models.UpsertRedirectParams{
		WorkspaceID: at.workspace, HelpCenterID: at.helpCenter, FromPath: from, TargetType: kind,
		TargetArticleID: to.article, TargetCollectionID: to.collection, Source: "slug_change", Now: now,
	})
	if err != nil {
		return fmt.Errorf("record redirect: %w", err)
	}
	return nil
}

/*
touch moves the help center's cache version on.
*/
func touch(ctx context.Context, q *models.Queries, at place, now time.Time) error {
	if err := q.TouchHelpCenterCache(ctx, models.TouchHelpCenterCacheParams{
		WorkspaceID: at.workspace, ID: at.helpCenter, Now: now,
	}); err != nil {
		return fmt.Errorf("touch help center cache: %w", err)
	}
	return nil
}
