/*
|--------------------------------------------------------------------------
| Translations
|--------------------------------------------------------------------------
|
| An article is written in its help center's default language, and that
| text is the source. A translation into another language is edited the
| way the source is: a working copy saved apart from what is live, a
| lock version so two editors never overwrite each other, and a publish
| that freezes it into a version readers see under /<language>/. A
| translation is outdated once the source is published with other words
| than the ones it last caught up with; publishing it again or marking
| it up to date catches it up. Publishing lives in
| translation_publishing.go, collections in collection_translations.go.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
Failures of managing translations.
*/
var (
	ErrTranslationNotFound = fault.New(fault.NotFound, "translation_not_found",
		"This article has no translation into this language yet.")
	ErrTranslationChanged = fault.New(fault.Conflict, "translation_changed",
		"Someone saved this translation since you opened it. Reload to see their changes.")
	ErrCollectionTranslationNotFound = fault.New(fault.NotFound, "collection_translation_not_found",
		"This collection has no translation into this language yet.")
)

/*
Translation is one article's translation into one language, with
whether the source changed since it last caught up.
*/
type Translation = models.FindArticleTranslationRow

/*
TranslationRef names an article's translation: the article and the
language.
*/
type TranslationRef struct {
	Article ids.UUID
	Locale  string
}

/*
Translations manages the translations of a workspace's articles and
collections. Index, when set, keeps the knowledge index in step with
what is published.
*/
type Translations struct {
	DB       *database.DB
	Clock    clock.Clock
	Renderer *markdown.Renderer
	Index    ArticleIndexer
}

/*
NewTranslations returns the translations action with the article
renderer.
*/
func NewTranslations(db *database.DB, clk clock.Clock) *Translations {
	return &Translations{DB: db, Clock: clk, Renderer: renderer()}
}

/*
TranslationInput is a translation to write: the article and language,
and the fields to change, nil leaving one as it is. A new translation
needs a title. LockVersion is the version the editor started from, zero
to skip the check.
*/
type TranslationInput struct {
	Membership  authz.Membership
	ArticleID   ids.UUID
	Locale      string
	Title       *string
	Slug        *string
	Description *string
	Body        *string
	LockVersion int32
}

/*
List returns an article's translations.
*/
func (t *Translations) List(ctx context.Context, m authz.Membership, articleID ids.UUID,
) ([]Translation, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []Translation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findArticle(ctx, q, m.Workspace.ID, articleID); err != nil {
			return err
		}
		rows, err := q.ListArticleTranslations(ctx, models.ListArticleTranslationsParams{
			WorkspaceID: m.Workspace.ID, ArticleID: articleID,
		})
		for _, r := range rows {
			out = append(out, Translation(r))
		}
		return err
	})
	return out, err
}

/*
Show returns an article's translation into one language.
*/
func (t *Translations) Show(ctx context.Context, m authz.Membership, ref TranslationRef) (Translation, error) {
	if !m.Can("articles:read") {
		return Translation{}, exceptions.ErrForbidden
	}
	var out Translation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findArticle(ctx, q, m.Workspace.ID, ref.Article); err != nil {
			return err
		}
		var err error
		out, err = findTranslation(ctx, q, m.Workspace.ID, ref)
		return err
	})
	return out, err
}

/*
Save writes an article's translation into a language, starting a draft
when there is none.
*/
func (t *Translations) Save(ctx context.Context, in TranslationInput) (Translation, error) {
	if !in.Membership.Can("articles:write") {
		return Translation{}, exceptions.ErrForbidden
	}
	ws := in.Membership.Workspace.ID
	var out Translation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		article, h, err := articleAndCenter(ctx, q, ws, in.ArticleID)
		if err != nil {
			return err
		}
		if in.Locale, err = checkTranslationLocale(h, in.Locale); err != nil {
			return err
		}
		if err := t.write(ctx, q, article, in); err != nil {
			return err
		}
		out, err = findTranslation(ctx, q, ws, TranslationRef{Article: article.ID, Locale: in.Locale})
		return err
	})
	return out, err
}

/*
write creates the translation into in.Locale or changes the one there
is.
*/
func (t *Translations) write(ctx context.Context, q *models.Queries, a models.Article, in TranslationInput) error {
	current, err := q.LockArticleTranslation(ctx, models.LockArticleTranslationParams{
		WorkspaceID: a.WorkspaceID, ArticleID: a.ID, Locale: in.Locale,
	})
	switch {
	case errors.Is(err, database.ErrNotFound):
		return t.create(ctx, q, a, in)
	case err != nil:
		return fmt.Errorf("lock translation: %w", err)
	}
	return t.change(ctx, q, current, in)
}

/*
create stores a new draft translation into in.Locale, caught up with
the source as it now is.
*/
func (t *Translations) create(ctx context.Context, q *models.Queries, a models.Article, in TranslationInput) error {
	d := draft{title: strings.TrimSpace(deref(in.Title))}
	if d.title == "" || len([]rune(d.title)) > 150 {
		return fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 150 characters.").OnField("title")
	}
	d.slug = translatedSlug(d.title, a.Slug, 120)
	if err := d.overlay(ArticleInput{Slug: in.Slug, Description: in.Description, Body: in.Body}); err != nil {
		return err
	}
	if err := t.checkSlug(ctx, q, a, slugIn{locale: in.Locale, slug: d.slug}); err != nil {
		return err
	}
	source, err := q.ArticleSourceHash(ctx, models.ArticleSourceHashParams{WorkspaceID: a.WorkspaceID, ID: a.ID})
	if err != nil {
		return fmt.Errorf("hash source: %w", err)
	}
	member, now := memberOf(in.Membership), t.Clock.Now()
	_, err = q.CreateArticleTranslation(ctx, models.CreateArticleTranslationParams{
		ID: ids.NewV7At(now), WorkspaceID: a.WorkspaceID, HelpCenterID: a.HelpCenterID, ArticleID: a.ID,
		Locale: in.Locale, Title: d.title, Slug: d.slug, Description: nonEmpty(d.description), BodyMarkdown: d.body,
		ContentHash: d.textHash(), SourceHash: &source, AuthorMemberID: member, EditedByType: new("member"),
		EditedByID: member, WordCount: wordCount(d.body), Now: now,
	})
	if err != nil {
		return fmt.Errorf("create translation: %w", err)
	}
	return nil
}

/*
change writes over a translation's working copy, refused when someone
saved since the editor started.
*/
func (t *Translations) change(ctx context.Context, q *models.Queries, c models.ArticleTranslation, in TranslationInput,
) error {
	if in.LockVersion != 0 && in.LockVersion != c.LockVersion {
		return ErrTranslationChanged
	}
	d, err := changedTranslation(c, in)
	if err != nil {
		return err
	}
	a := models.Article{ID: c.ArticleID, WorkspaceID: c.WorkspaceID, HelpCenterID: c.HelpCenterID}
	if err := t.checkSlug(ctx, q, a, slugIn{locale: c.Locale, slug: d.slug}); err != nil {
		return err
	}
	published, err := translationPublishedHash(ctx, q, c)
	if err != nil {
		return err
	}
	member := memberOf(in.Membership)
	_, err = q.UpdateArticleTranslation(ctx, models.UpdateArticleTranslationParams{
		Title: d.title, Slug: d.slug, Description: nonEmpty(d.description), BodyMarkdown: d.body,
		ContentHash: d.textHash(), HasUnpublishedChanges: d.textHash() != published, EditedByType: new("member"),
		EditedByID: member, WordCount: wordCount(d.body), Now: t.Clock.Now(), WorkspaceID: c.WorkspaceID, ID: c.ID,
		LockVersion: c.LockVersion,
	})
	if errors.Is(err, database.ErrNotFound) {
		return ErrTranslationChanged
	}
	if err != nil {
		return fmt.Errorf("save translation: %w", err)
	}
	return nil
}

/*
changedTranslation writes a change over a translation's working copy.
*/
func changedTranslation(c models.ArticleTranslation, in TranslationInput) (draft, error) {
	d := draft{title: c.Title, slug: c.Slug, description: deref(c.Description), body: c.BodyMarkdown}
	if in.Title != nil {
		d.title = strings.TrimSpace(*in.Title)
		if d.title == "" || len([]rune(d.title)) > 150 {
			return d, fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 150 characters.").OnField("title")
		}
	}
	err := d.overlay(ArticleInput{Slug: in.Slug, Description: in.Description, Body: in.Body})
	return d, err
}

/*
slugIn is a slug an article's translation into a language would take.
*/
type slugIn struct {
	locale string
	slug   string
}

/*
checkSlug refuses a slug another article's translation into the
language holds, first freeing it from articles in the trash.
*/
func (t *Translations) checkSlug(ctx context.Context, q *models.Queries, a models.Article, s slugIn) error {
	if err := q.FreeArticleTranslationSlug(ctx, models.FreeArticleTranslationSlugParams{
		Now: t.Clock.Now(), WorkspaceID: a.WorkspaceID, HelpCenterID: a.HelpCenterID, Locale: s.locale, Slug: s.slug,
	}); err != nil {
		return fmt.Errorf("free translation slug: %w", err)
	}
	taken, err := q.ArticleTranslationSlugTaken(ctx, models.ArticleTranslationSlugTakenParams{
		WorkspaceID: a.WorkspaceID, HelpCenterID: a.HelpCenterID, Locale: s.locale, Slug: s.slug, ArticleID: a.ID,
	})
	if err != nil {
		return fmt.Errorf("check translation slug: %w", err)
	}
	if taken {
		return ErrSlugTaken
	}
	return nil
}
