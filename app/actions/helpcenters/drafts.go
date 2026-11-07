/*
|--------------------------------------------------------------------------
| Article Drafts
|--------------------------------------------------------------------------
|
| What a save may write to an article's working copy: a title, a slug
| made from it when none is given, a description, the Markdown body, who
| may read it and its collection, each checked before anything is
| stored.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxBody is the largest article body, in bytes.
*/
const maxBody = 256 << 10

/*
newDraft reads a new article's fields.
*/
func newDraft(in ArticleInput) (draft, error) {
	d := draft{visibility: "public", collection: in.CollectionID}
	d.title = strings.TrimSpace(deref(in.Title))
	if d.title == "" || len([]rune(d.title)) > 150 {
		return d, fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 150 characters.").OnField("title")
	}
	d.slug = Slugify(d.title, 120)
	if d.slug == "" {
		d.slug = "article"
	}
	err := d.overlay(in)
	return d, err
}

/*
changedDraft writes a change over an article's working copy.
*/
func changedDraft(c models.Article, in ArticleInput) (draft, error) {
	d := draft{
		title: c.Title, slug: c.Slug, description: deref(c.Description), body: c.BodyMarkdown,
		visibility: c.Visibility, collection: c.CollectionID,
	}
	if in.Title != nil {
		d.title = strings.TrimSpace(*in.Title)
		if d.title == "" || len([]rune(d.title)) > 150 {
			return d, fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 150 characters.").OnField("title")
		}
	}
	if in.ClearCollection {
		d.collection = nil
	}
	if in.CollectionID != nil {
		d.collection = in.CollectionID
	}
	err := d.overlay(in)
	return d, err
}

/*
overlay writes the fields every save may send.
*/
func (d *draft) overlay(in ArticleInput) error {
	if in.Slug != nil && strings.TrimSpace(*in.Slug) != "" {
		d.slug = strings.ToLower(strings.TrimSpace(*in.Slug))
	}
	if len(d.slug) > 120 || !slugPattern().MatchString(d.slug) {
		return ErrSlugInvalid
	}
	if in.Description != nil {
		if d.description = strings.TrimSpace(*in.Description); len([]rune(d.description)) > 300 {
			return fault.New(fault.Invalid, "description_too_long", "Descriptions are at most 300 characters.").
				OnField("description")
		}
	}
	if in.Body != nil {
		if d.body = *in.Body; len(d.body) > maxBody {
			return fault.New(fault.TooLarge, "body_too_large", "Articles are at most 256 KB.").OnField("body_markdown")
		}
	}
	if in.Visibility != nil {
		if *in.Visibility != "public" && *in.Visibility != "agent_only" {
			return fault.New(fault.Invalid, "visibility_invalid", "Visibility is public or agent_only.").
				OnField("visibility")
		}
		d.visibility = *in.Visibility
	}
	return nil
}

/*
placement is where a save puts an article: its help center, the article
itself, empty for a new one, and the draft being saved.
*/
type placement struct {
	at    place
	self  ids.UUID
	draft *draft
}

/*
checkPlace refuses a collection of another help center and a slug
another article of the help center holds.
*/
func (a *Articles) checkPlace(ctx context.Context, q *models.Queries, p placement) error {
	ws, helpCenter, self, d := p.at.workspace, p.at.helpCenter, p.self, p.draft
	if d.collection != nil {
		col, err := findCollection(ctx, q, ws, *d.collection)
		if err != nil {
			return err
		}
		if col.HelpCenterID != helpCenter {
			return ErrCollectionNotFound
		}
	}
	taken, err := q.ArticleSlugTaken(ctx, models.ArticleSlugTakenParams{
		WorkspaceID: ws, HelpCenterID: helpCenter, Slug: d.slug, ID: self,
	})
	if err != nil {
		return fmt.Errorf("check article slug: %w", err)
	}
	if taken {
		return ErrSlugTaken
	}
	return nil
}

/*
publishedHash is the content hash of the live version, empty for an
article never published.
*/
func publishedHash(ctx context.Context, q *models.Queries, c models.Article) (string, error) {
	if c.PublishedVersionID == nil {
		return "", nil
	}
	v, err := q.FindArticleVersion(ctx, models.FindArticleVersionParams{
		WorkspaceID: c.WorkspaceID, ID: *c.PublishedVersionID,
	})
	if err != nil {
		return "", fmt.Errorf("find published version: %w", err)
	}
	return v.ContentHash, nil
}

/*
findArticle returns a live article of the workspace, or 404.
*/
func findArticle(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Article, error) {
	out, err := q.FindArticle(ctx, models.FindArticleParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrArticleNotFound
	}
	if err != nil {
		return out, fmt.Errorf("find article: %w", err)
	}
	return out, nil
}

/*
lockArticle returns a live article and holds it until the transaction
ends, or 404.
*/
func lockArticle(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Article, error) {
	out, err := q.LockArticle(ctx, models.LockArticleParams{WorkspaceID: ws, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrArticleNotFound
	}
	if err != nil {
		return out, fmt.Errorf("lock article: %w", err)
	}
	return out, nil
}

func memberOf(m authz.Membership) *ids.UUID {
	if m.Member.ID == (ids.UUID{}) {
		return nil
	}
	return &m.Member.ID
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
