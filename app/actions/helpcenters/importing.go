/*
|--------------------------------------------------------------------------
| Imported Articles
|--------------------------------------------------------------------------
|
| Another tool's help center, or a folder of Markdown, becomes ordinary
| editable articles here. Collections are matched by name under their
| parent, so a second import reuses what the first made; trees deeper
| than the help center allows are folded into their deepest allowed
| level. An imported article is a working copy edited by the system, and
| published at once when its source had it published, through the same
| publishing as an editor's, so it is rendered, indexed and searchable.
| Its old address keeps working as a permanent redirect, so links
| customers saved still land on the right page once the domain moves.
| These helpers run inside the caller's transaction and trust it to have
| checked who asked.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ImportCollectionInput is one collection of an import: where it goes,
under which parent, and what it is called.
*/
type ImportCollectionInput struct {
	WorkspaceID  ids.UUID
	HelpCenterID ids.UUID
	ParentID     *ids.UUID
	Name         string
	Description  string
}

/*
ImportCollection returns the collection of the name under the parent,
creating it when there is none.
*/
func (c *Collections) ImportCollection(ctx context.Context, tx *database.Tx, in ImportCollectionInput,
) (ids.UUID, error) {
	q := models.New(tx)
	name := truncate(strings.TrimSpace(in.Name), 80)
	if name == "" {
		name = "Imported"
	}
	found, err := q.FindCollectionForImport(ctx, models.FindCollectionForImportParams{
		WorkspaceID: in.WorkspaceID, HelpCenterID: in.HelpCenterID, ParentID: in.ParentID, Name: name,
	})
	if err == nil {
		return found, nil
	}
	if !errors.Is(err, database.ErrNotFound) {
		return ids.UUID{}, fmt.Errorf("find imported collection: %w", err)
	}
	now := c.Clock.Now()
	slug, err := freeCollectionSlug(ctx, q, in, name)
	if err != nil {
		return ids.UUID{}, err
	}
	position, err := q.NextCollectionPosition(ctx, models.NextCollectionPositionParams{
		WorkspaceID: in.WorkspaceID, HelpCenterID: in.HelpCenterID,
	})
	if err != nil {
		return ids.UUID{}, fmt.Errorf("place collection: %w", err)
	}
	created, err := q.CreateCollection(ctx, models.CreateCollectionParams{
		ID: ids.NewV7At(now), WorkspaceID: in.WorkspaceID, HelpCenterID: in.HelpCenterID, ParentID: in.ParentID,
		Name: name, Slug: slug, Description: optional(&in.Description, 300), Position: position, Now: now,
	})
	if err != nil {
		return ids.UUID{}, fmt.Errorf("create imported collection: %w", err)
	}
	return created.ID, touch(ctx, q, place{workspace: in.WorkspaceID, helpCenter: in.HelpCenterID}, now)
}

/*
freeCollectionSlug is the name's slug, numbered when another
collection of the help center holds it.
*/
func freeCollectionSlug(ctx context.Context, q *models.Queries, in ImportCollectionInput, name string,
) (string, error) {
	base := Slugify(name, 70)
	if base == "" {
		base = "collection"
	}
	for n := 1; n < 100; n++ {
		slug := base
		if n > 1 {
			slug += "-" + strconv.Itoa(n)
		}
		_, err := q.FindCollectionBySlug(ctx, models.FindCollectionBySlugParams{
			WorkspaceID: in.WorkspaceID, HelpCenterID: in.HelpCenterID, Slug: slug,
		})
		if errors.Is(err, database.ErrNotFound) {
			return slug, nil
		}
		if err != nil {
			return "", fmt.Errorf("check collection slug: %w", err)
		}
	}
	return "", ErrSlugTaken
}

/*
ImportArticleInput is one article of an import. ArticleID names the
article an earlier import made, to update it rather than make another;
Publish publishes it when its source had it published.
*/
type ImportArticleInput struct {
	WorkspaceID  ids.UUID
	HelpCenterID ids.UUID
	CollectionID *ids.UUID
	ArticleID    *ids.UUID
	Title        string
	Description  string
	Body         string
	Visibility   string
	Publish      bool
}

/*
ImportArticle writes the article's working copy, a new one or the
earlier import's, and publishes it when asked and it changed.
*/
func (a *Articles) ImportArticle(ctx context.Context, tx *database.Tx, in ImportArticleInput) (models.Article, error) {
	q := models.New(tx)
	art, err := a.importDraft(ctx, q, in)
	if err != nil || !in.Publish {
		return art, err
	}
	if art.Status == "published" && !art.HasUnpublishedChanges {
		return art, nil
	}
	published, err := a.freeze(ctx, q, art, nil)
	if err != nil {
		return art, err
	}
	at := place{workspace: in.WorkspaceID, helpCenter: in.HelpCenterID}
	return published.article, a.settle(ctx, tx, at, art.ID)
}

/*
importDraft saves the working copy, keeping the earlier import's
article when it still exists.
*/
func (a *Articles) importDraft(ctx context.Context, q *models.Queries, in ImportArticleInput) (models.Article, error) {
	member := authz.Membership{Workspace: models.Workspace{ID: in.WorkspaceID}}
	body, description, visibility := in.Body, in.Description, in.Visibility
	title := truncate(strings.TrimSpace(in.Title), 150)
	if title == "" {
		title = "Untitled"
	}
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	description = truncate(description, 300)
	input := ArticleInput{
		Membership: member, HelpCenterID: in.HelpCenterID, CollectionID: in.CollectionID, Title: &title,
		Body: &body, Description: &description, Visibility: &visibility,
	}
	if in.ArticleID != nil {
		if current, err := lockArticle(ctx, q, in.WorkspaceID, *in.ArticleID); err == nil {
			input.ID = current.ID
			return a.saveImported(ctx, q, current, input)
		}
	}
	return a.createImported(ctx, q, input)
}

/*
createImported makes a new draft with a free slug.
*/
func (a *Articles) createImported(ctx context.Context, q *models.Queries, in ArticleInput) (models.Article, error) {
	d, err := newDraft(in)
	if err != nil {
		return models.Article{}, err
	}
	ws, now := in.Membership.Workspace.ID, a.Clock.Now()
	at := place{workspace: ws, helpCenter: in.HelpCenterID}
	if d.slug, err = a.freeArticleSlug(ctx, q, at, d); err != nil {
		return models.Article{}, err
	}
	position, err := q.NextArticlePosition(ctx, models.NextArticlePositionParams{
		WorkspaceID: ws, HelpCenterID: in.HelpCenterID, CollectionID: d.collection,
	})
	if err != nil {
		return models.Article{}, fmt.Errorf("place article: %w", err)
	}
	out, err := q.CreateArticle(ctx, models.CreateArticleParams{
		ID: ids.NewV7At(now), WorkspaceID: ws, HelpCenterID: in.HelpCenterID, CollectionID: d.collection,
		Title: d.title, Slug: d.slug, Description: nonEmpty(d.description), BodyMarkdown: d.body,
		Visibility: d.visibility, ContentHash: d.hash(), Position: position, EditedByType: new("system"),
		WordCount: wordCount(d.body), Now: now,
	})
	if err != nil {
		return out, fmt.Errorf("create imported article: %w", err)
	}
	return out, nil
}

/*
saveImported writes the new text over the earlier import's working
copy, keeping its slug.
*/
func (a *Articles) saveImported(ctx context.Context, q *models.Queries, current models.Article, in ArticleInput,
) (models.Article, error) {
	d, err := changedDraft(current, in)
	if err != nil {
		return current, err
	}
	published, err := publishedHash(ctx, q, current)
	if err != nil {
		return current, err
	}
	out, err := q.UpdateArticle(ctx, models.UpdateArticleParams{
		CollectionID: d.collection, Title: d.title, Slug: d.slug, Description: nonEmpty(d.description),
		BodyMarkdown: d.body, Visibility: d.visibility, ContentHash: d.hash(),
		HasUnpublishedChanges: d.hash() != published, EditedByType: new("system"), WordCount: wordCount(d.body),
		Now: a.Clock.Now(), WorkspaceID: current.WorkspaceID, ID: current.ID, LockVersion: current.LockVersion,
	})
	if err != nil {
		return out, fmt.Errorf("save imported article: %w", err)
	}
	return out, nil
}

/*
freeArticleSlug is the draft's slug, numbered when another article of
the help center holds it.
*/
func (a *Articles) freeArticleSlug(ctx context.Context, q *models.Queries, at place, d draft) (string, error) {
	base := d.slug
	for n := 1; n < 100; n++ {
		d.slug = base
		if n > 1 {
			d.slug = truncate(base, 110) + "-" + strconv.Itoa(n)
		}
		err := a.checkPlace(ctx, q, placement{at: at, draft: &d})
		if err == nil {
			return d.slug, nil
		}
		if !errors.Is(err, ErrSlugTaken) {
			return "", err
		}
	}
	return "", ErrSlugTaken
}

/*
ImportRedirect makes an address of the old help center lead to the
imported article for good.
*/
func (a *Articles) ImportRedirect(ctx context.Context, tx *database.Tx, art models.Article, fromPath string) error {
	if fromPath == "" || fromPath == "/" || len(fromPath) > 512 {
		return nil
	}
	err := models.New(tx).UpsertRedirect(ctx, models.UpsertRedirectParams{
		WorkspaceID: art.WorkspaceID, HelpCenterID: art.HelpCenterID, FromPath: fromPath, TargetType: "article",
		TargetArticleID: &art.ID, Source: "import", Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("record import redirect: %w", err)
	}
	return nil
}

/*
HelpCenterLocales maps each live help center of the workspace to its
language, so translations land in the help center of theirs.
*/
func HelpCenterLocales(ctx context.Context, tx *database.Tx, ws ids.UUID) (map[ids.UUID]string, error) {
	rows, err := models.New(tx).ListHelpCenterLocales(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("list help center locales: %w", err)
	}
	out := make(map[ids.UUID]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.Locale
	}
	return out, nil
}
