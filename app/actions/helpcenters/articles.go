/*
|--------------------------------------------------------------------------
| Articles
|--------------------------------------------------------------------------
|
| An article is edited as a working copy and read as its published
| version. Saving never changes what readers see; publishing freezes the
| working copy into a new version, rendered once, and points the article
| at it. Two editors cannot overwrite each other: a save names the lock
| version it started from and is refused when someone saved since.
| Unpublishing and archiving keep every version, and a published slug
| that changes leaves a redirect from the old address.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Failures of managing articles.
*/
var (
	ErrArticleNotFound = fault.New(fault.NotFound, "article_not_found",
		"No article of this workspace has this id.")
	ErrArticleChanged = fault.New(fault.Conflict, "article_changed",
		"Someone saved this article since you opened it. Reload to see their changes.")
	ErrNothingToPublish = fault.New(fault.Conflict, "nothing_to_publish",
		"This article has no changes to publish.")
)

/*
Articles manages articles.
*/
type Articles struct {
	DB       *database.DB
	Clock    clock.Clock
	Renderer *markdown.Renderer
}

/*
NewArticles returns the articles action with the article renderer.
*/
func NewArticles(db *database.DB, clk clock.Clock) *Articles {
	return &Articles{DB: db, Clock: clk, Renderer: renderer()}
}

/*
ArticleFilter narrows a list of a help center's articles.
*/
type ArticleFilter struct {
	HelpCenterID ids.UUID
	Status       string
	CollectionID *ids.UUID
	Query        string
	Limit        int32
}

/*
ArticleInput is an article to create or a change to one. On change, nil
leaves a field as it is, ClearCollection uncollects it, and LockVersion
is the version the editor started from.
*/
type ArticleInput struct {
	Membership      authz.Membership
	HelpCenterID    ids.UUID
	ID              ids.UUID
	CollectionID    *ids.UUID
	ClearCollection bool
	Title           *string
	Slug            *string
	Description     *string
	Body            *string
	Visibility      *string
	LockVersion     int32
}

/*
List returns a help center's articles.
*/
func (a *Articles) List(ctx context.Context, m authz.Membership, f ArticleFilter) ([]models.Article, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var out []models.Article
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListArticles(ctx, models.ListArticlesParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: f.HelpCenterID, Status: nonEmpty(f.Status),
			CollectionID: f.CollectionID, Query: nonEmpty(strings.TrimSpace(f.Query)), RowLimit: limit,
		})
		return err
	})
	return out, err
}

/*
Show returns one article.
*/
func (a *Articles) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.Article, error) {
	if !m.Can("articles:read") {
		return models.Article{}, exceptions.ErrForbidden
	}
	var out models.Article
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = findArticle(ctx, models.New(tx), m.Workspace.ID, id)
		return err
	})
	return out, err
}

/*
Create makes a draft at the end of its collection.
*/
func (a *Articles) Create(ctx context.Context, in ArticleInput) (models.Article, error) {
	if !in.Membership.Can("articles:write") {
		return models.Article{}, exceptions.ErrForbidden
	}
	ws, now := in.Membership.Workspace.ID, a.Clock.Now()
	var out models.Article
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findHelpCenter(ctx, q, ws, in.HelpCenterID); err != nil {
			return err
		}
		d, err := newDraft(in)
		if err != nil {
			return err
		}
		at := place{workspace: ws, helpCenter: in.HelpCenterID}
		if err := a.checkPlace(ctx, q, placement{at: at, draft: &d}); err != nil {
			return err
		}
		position, err := q.NextArticlePosition(ctx, models.NextArticlePositionParams{
			WorkspaceID: ws, HelpCenterID: in.HelpCenterID, CollectionID: d.collection,
		})
		if err != nil {
			return fmt.Errorf("place article: %w", err)
		}
		member := memberOf(in.Membership)
		out, err = q.CreateArticle(ctx, models.CreateArticleParams{
			ID: ids.NewV7At(now), WorkspaceID: ws, HelpCenterID: in.HelpCenterID, CollectionID: d.collection,
			Title: d.title, Slug: d.slug, Description: nonEmpty(d.description), BodyMarkdown: d.body,
			Visibility: d.visibility, ContentHash: d.hash(), Position: position, AuthorMemberID: member,
			EditedByType: new("member"), EditedByID: member, WordCount: wordCount(d.body), Now: now,
		})
		if err != nil {
			return fmt.Errorf("create article: %w", err)
		}
		return nil
	})
	return out, err
}

/*
Update saves the working copy.
*/
func (a *Articles) Update(ctx context.Context, in ArticleInput) (models.Article, error) {
	if !in.Membership.Can("articles:write") {
		return models.Article{}, exceptions.ErrForbidden
	}
	now := a.Clock.Now()
	var out models.Article
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = a.save(ctx, models.New(tx), in, now)
		return err
	})
	return out, err
}

/*
save writes a change over the locked working copy, refused when someone
saved since the editor started.
*/
func (a *Articles) save(ctx context.Context, q *models.Queries, in ArticleInput, now time.Time,
) (models.Article, error) {
	ws := in.Membership.Workspace.ID
	current, err := lockArticle(ctx, q, ws, in.ID)
	if err != nil {
		return models.Article{}, err
	}
	if in.LockVersion != 0 && in.LockVersion != current.LockVersion {
		return models.Article{}, ErrArticleChanged
	}
	d, err := changedDraft(current, in)
	if err != nil {
		return models.Article{}, err
	}
	at := place{workspace: ws, helpCenter: current.HelpCenterID}
	if err := a.checkPlace(ctx, q, placement{at: at, self: current.ID, draft: &d}); err != nil {
		return models.Article{}, err
	}
	published, err := publishedHash(ctx, q, current)
	if err != nil {
		return models.Article{}, err
	}
	member := memberOf(in.Membership)
	out, err := q.UpdateArticle(ctx, models.UpdateArticleParams{
		CollectionID: d.collection, Title: d.title, Slug: d.slug, Description: nonEmpty(d.description),
		BodyMarkdown: d.body, Visibility: d.visibility, ContentHash: d.hash(),
		HasUnpublishedChanges: d.hash() != published, EditedByType: new("member"), EditedByID: member,
		WordCount: wordCount(d.body), Now: now, WorkspaceID: ws, ID: current.ID, LockVersion: current.LockVersion,
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrArticleChanged
	}
	if err != nil {
		return out, fmt.Errorf("save article: %w", err)
	}
	return out, nil
}

/*
Publish freezes the working copy into a new version and makes it live.
*/
func (a *Articles) Publish(ctx context.Context, m authz.Membership, id ids.UUID) (models.Article, error) {
	if !m.Can("articles:write") {
		return models.Article{}, exceptions.ErrForbidden
	}
	ws, now := m.Workspace.ID, a.Clock.Now()
	var out models.Article
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lockArticle(ctx, q, ws, id)
		if err != nil {
			return err
		}
		if current.Status == "published" && !current.HasUnpublishedChanges {
			return ErrNothingToPublish
		}
		previous, err := a.freeze(ctx, q, current, memberOf(m))
		if err != nil {
			return err
		}
		out = previous.article
		if previous.slug != "" && previous.slug != out.Slug {
			at := place{workspace: ws, helpCenter: out.HelpCenterID}
			m := move{from: "/articles/" + previous.slug, to: target{article: &out.ID}, now: now}
			if err := redirect(ctx, q, at, m); err != nil {
				return err
			}
		}
		return touch(ctx, q, place{workspace: ws, helpCenter: out.HelpCenterID}, now)
	})
	return out, err
}

/*
frozen is a publish: the article as it now is and the slug its previous
version had, empty when it had none.
*/
type frozen struct {
	article models.Article
	slug    string
}

/*
freeze renders the working copy into a version and points the article
at it.
*/
func (a *Articles) freeze(ctx context.Context, q *models.Queries, c models.Article, member *ids.UUID) (frozen, error) {
	now, out := a.Clock.Now(), frozen{}
	if c.PublishedVersionID != nil {
		prev, err := q.FindArticleVersion(ctx, models.FindArticleVersionParams{
			WorkspaceID: c.WorkspaceID, ID: *c.PublishedVersionID,
		})
		if err == nil {
			out.slug = prev.Slug
		}
	}
	body, err := render(a.Renderer, c.BodyMarkdown)
	if err != nil {
		return out, err
	}
	number, err := q.NextArticleVersionNumber(ctx, models.NextArticleVersionNumberParams{
		WorkspaceID: c.WorkspaceID, ArticleID: c.ID,
	})
	if err != nil {
		return out, fmt.Errorf("number article version: %w", err)
	}
	version, err := q.CreateArticleVersion(ctx, models.CreateArticleVersionParams{
		ID: ids.NewV7At(now), WorkspaceID: c.WorkspaceID, ArticleID: c.ID, Number: number, Kind: "published",
		Title: c.Title, Slug: c.Slug, Description: c.Description, BodyMarkdown: c.BodyMarkdown, BodyHtml: body.html,
		Toc: body.toc, Visibility: c.Visibility, CollectionID: c.CollectionID, ContentHash: c.ContentHash,
		CreatedByType: "member", CreatedByID: member, RendererVersion: RendererVersion, Now: now,
	})
	if err != nil {
		return out, fmt.Errorf("freeze article version: %w", err)
	}
	out.article, err = q.PublishArticle(ctx, models.PublishArticleParams{
		VersionID: &version.ID, Now: now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return out, fmt.Errorf("publish article: %w", err)
	}
	return out, nil
}

/*
SetStatus unpublishes an article back to a draft or archives it; its
versions stay.
*/
func (a *Articles) SetStatus(ctx context.Context, m authz.Membership, id ids.UUID, status string,
) (models.Article, error) {
	if !m.Can("articles:write") {
		return models.Article{}, exceptions.ErrForbidden
	}
	if status != "draft" && status != "archived" {
		return models.Article{}, fault.New(fault.Invalid, "status_invalid",
			"Publish to make an article live; otherwise the status is draft or archived.").OnField("status")
	}
	ws, now := m.Workspace.ID, a.Clock.Now()
	var out models.Article
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := lockArticle(ctx, q, ws, id); err != nil {
			return err
		}
		var err error
		out, err = q.SetArticleStatus(ctx, models.SetArticleStatusParams{
			Status: status, Now: now, WorkspaceID: ws, ID: id,
		})
		if err != nil {
			return fmt.Errorf("set article status: %w", err)
		}
		return touch(ctx, q, place{workspace: ws, helpCenter: out.HelpCenterID}, now)
	})
	return out, err
}

/*
Delete moves an article to the trash.
*/
func (a *Articles) Delete(ctx context.Context, m authz.Membership, id ids.UUID) error {
	if !m.Can("articles:write") {
		return exceptions.ErrForbidden
	}
	ws, now := m.Workspace.ID, a.Clock.Now()
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := lockArticle(ctx, q, ws, id)
		if err != nil {
			return err
		}
		if err := q.DeleteArticle(ctx, models.DeleteArticleParams{WorkspaceID: ws, ID: id, Now: now}); err != nil {
			return fmt.Errorf("delete article: %w", err)
		}
		return touch(ctx, q, place{workspace: ws, helpCenter: current.HelpCenterID}, now)
	})
}

/*
Versions lists an article's versions, newest first.
*/
func (a *Articles) Versions(ctx context.Context, m authz.Membership, id ids.UUID,
) ([]models.ListArticleVersionsRow, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.ListArticleVersionsRow
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findArticle(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}
		var err error
		out, err = q.ListArticleVersions(ctx, models.ListArticleVersionsParams{
			WorkspaceID: m.Workspace.ID, ArticleID: id,
		})
		return err
	})
	return out, err
}
