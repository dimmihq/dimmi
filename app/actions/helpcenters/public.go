/*
|--------------------------------------------------------------------------
| What Readers See
|--------------------------------------------------------------------------
|
| A reader's request names a help center by its subdomain before anyone
| knows the workspace, so that one lookup runs through a security
| definer function; everything after reads inside the workspace. A help
| center's published articles and collections are read once per cache
| version and kept in memory, so pages are served without a query until
| something a reader could see changes. Only published public articles
| are ever in it: drafts, archived and agent-only articles never are.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/memo"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Snapshot is a help center as readers see it at one cache version.
*/
type Snapshot struct {
	HelpCenter  models.HelpCenter
	Collections []models.Collection
	Articles    []models.ListPublishedArticlesRow
}

/*
snapshotKey names a help center at one cache version.
*/
type snapshotKey struct {
	id      ids.UUID
	version int64
}

/*
Public reads help centers for readers.
*/
type Public struct {
	DB        *database.DB
	snapshots *memo.Cache[snapshotKey, Snapshot]
}

/*
NewPublic returns a reader keeping up to a thousand help centers for a
minute after their last change was read.
*/
func NewPublic(db *database.DB) *Public {
	return &Public{DB: db, snapshots: memo.New[snapshotKey, Snapshot](memo.Options{Size: 1000, TTL: time.Minute})}
}

/*
ErrNoHelpCenter says no active help center answers the subdomain.
*/
var ErrNoHelpCenter = errors.New("help center not found")

/*
Load returns the help center answering the subdomain as readers see it.
*/
func (p *Public) Load(ctx context.Context, subdomain string) (Snapshot, error) {
	row, err := models.New(p.DB).ResolveHelpCenter(ctx, subdomain)
	if errors.Is(err, database.ErrNotFound) {
		return Snapshot{}, ErrNoHelpCenter
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("resolve help center: %w", err)
	}
	var out Snapshot
	scoped := tenancy.With(ctx, row.WorkspaceID.String())
	err = p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams(row))
		if err != nil {
			return fmt.Errorf("find help center: %w", err)
		}
		key := snapshotKey{id: h.ID, version: h.CacheVersion}
		if cached, ok := p.snapshots.Get(key); ok {
			out = cached
			return nil
		}
		out, err = load(ctx, q, h)
		if err == nil {
			p.snapshots.Set(key, out)
		}
		return err
	})
	return out, err
}

/*
load reads a help center's collections and published articles.
*/
func load(ctx context.Context, q *models.Queries, h models.HelpCenter) (Snapshot, error) {
	out := Snapshot{HelpCenter: h}
	var err error
	out.Collections, err = q.ListCollections(ctx, models.ListCollectionsParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil {
		return out, fmt.Errorf("list collections: %w", err)
	}
	out.Articles, err = q.ListPublishedArticles(ctx, models.ListPublishedArticlesParams{
		WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID,
	})
	if err != nil {
		return out, fmt.Errorf("list published articles: %w", err)
	}
	return out, nil
}

/*
Search finds a help center's published public articles matching the
query, best first.
*/
func (p *Public) Search(ctx context.Context, h models.HelpCenter, query string, limit int32,
) ([]models.SearchPublishedArticlesRow, error) {
	var out []models.SearchPublishedArticlesRow
	scoped := tenancy.With(ctx, h.WorkspaceID.String())
	err := p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).SearchPublishedArticles(ctx, models.SearchPublishedArticlesParams{
			Query: query, WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, IncludeAgentOnly: false, RowLimit: limit,
		})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("search articles: %w", err)
	}
	return out, nil
}

/*
Redirect returns the path an old address of the help center now leads
to, relative to the help center's root.
*/
func (p *Public) Redirect(ctx context.Context, h models.HelpCenter, from string) (string, bool) {
	var to string
	scoped := tenancy.With(ctx, h.WorkspaceID.String())
	_ = p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).FindRedirect(ctx, models.FindRedirectParams{
			WorkspaceID: h.WorkspaceID, HelpCenterID: h.ID, FromPath: from,
		})
		if err != nil {
			return err
		}
		switch {
		case row.ArticleSlug != nil:
			to = "/articles/" + *row.ArticleSlug
		case row.CollectionSlug != nil:
			to = "/collections/" + *row.CollectionSlug
		case row.TargetUrl != nil:
			to = *row.TargetUrl
		}
		return nil
	})
	return to, to != ""
}

/*
LoadByID returns a workspace's help center as readers see it, for the
messenger, which knows its help center by id.
*/
func (p *Public) LoadByID(ctx context.Context, workspaceID, id ids.UUID) (Snapshot, error) {
	var out Snapshot
	scoped := tenancy.With(ctx, workspaceID.String())
	err := p.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := q.FindHelpCenter(ctx, models.FindHelpCenterParams{WorkspaceID: workspaceID, ID: id})
		if errors.Is(err, database.ErrNotFound) || (err == nil && h.Status != "active") {
			return ErrNoHelpCenter
		}
		if err != nil {
			return fmt.Errorf("find help center: %w", err)
		}
		key := snapshotKey{id: h.ID, version: h.CacheVersion}
		if cached, ok := p.snapshots.Get(key); ok {
			out = cached
			return nil
		}
		out, err = load(ctx, q, h)
		if err == nil {
			p.snapshots.Set(key, out)
		}
		return err
	})
	return out, err
}
