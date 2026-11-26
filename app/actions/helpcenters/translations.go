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
| it up to date catches it up. A published slug that changes leaves a
| redirect in its language. Collections get a translated name, slug and
| description, which readers of that language see once it holds one of
| their articles.
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
	"github.com/dimmihq/dimmi/framework/hashing"
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
func (t *Translations) Show(ctx context.Context, m authz.Membership, articleID ids.UUID, locale string,
) (Translation, error) {
	if !m.Can("articles:read") {
		return Translation{}, exceptions.ErrForbidden
	}
	var out Translation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findArticle(ctx, q, m.Workspace.ID, articleID); err != nil {
			return err
		}
		var err error
		out, err = findTranslation(ctx, q, m.Workspace.ID, articleID, canonicalLocale(locale))
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
		locale, err := checkTranslationLocale(h, in.Locale)
		if err != nil {
			return err
		}
		current, err := q.LockArticleTranslation(ctx, models.LockArticleTranslationParams{
			WorkspaceID: ws, ArticleID: article.ID, Locale: locale,
		})
		switch {
		case errors.Is(err, database.ErrNotFound):
			err = t.create(ctx, q, article, locale, in)
		case err != nil:
			return fmt.Errorf("lock translation: %w", err)
		default:
			err = t.change(ctx, q, current, in)
		}
		if err != nil {
			return err
		}
		out, err = findTranslation(ctx, q, ws, article.ID, locale)
		return err
	})
	return out, err
}

/*
create stores a new draft translation, caught up with the source as it
now is.
*/
func (t *Translations) create(ctx context.Context, q *models.Queries, a models.Article, locale string,
	in TranslationInput,
) error {
	d := draft{title: strings.TrimSpace(deref(in.Title))}
	if d.title == "" || len([]rune(d.title)) > 150 {
		return fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 150 characters.").OnField("title")
	}
	d.slug = Slugify(d.title, 120)
	if d.slug == "" {
		d.slug = a.Slug
	}
	if err := d.overlay(ArticleInput{Slug: in.Slug, Description: in.Description, Body: in.Body}); err != nil {
		return err
	}
	if err := t.checkSlug(ctx, q, a, locale, d.slug); err != nil {
		return err
	}
	source, err := q.ArticleSourceHash(ctx, models.ArticleSourceHashParams{WorkspaceID: a.WorkspaceID, ID: a.ID})
	if err != nil {
		return fmt.Errorf("hash source: %w", err)
	}
	member, now := memberOf(in.Membership), t.Clock.Now()
	_, err = q.CreateArticleTranslation(ctx, models.CreateArticleTranslationParams{
		ID: ids.NewV7At(now), WorkspaceID: a.WorkspaceID, HelpCenterID: a.HelpCenterID, ArticleID: a.ID,
		Locale: locale, Title: d.title, Slug: d.slug, Description: nonEmpty(d.description), BodyMarkdown: d.body,
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
	d := draft{title: c.Title, slug: c.Slug, description: deref(c.Description), body: c.BodyMarkdown}
	if in.Title != nil {
		d.title = strings.TrimSpace(*in.Title)
		if d.title == "" || len([]rune(d.title)) > 150 {
			return fault.New(fault.Invalid, "title_invalid", "Titles are 1 to 150 characters.").OnField("title")
		}
	}
	if err := d.overlay(ArticleInput{Slug: in.Slug, Description: in.Description, Body: in.Body}); err != nil {
		return err
	}
	a := models.Article{ID: c.ArticleID, WorkspaceID: c.WorkspaceID, HelpCenterID: c.HelpCenterID}
	if err := t.checkSlug(ctx, q, a, c.Locale, d.slug); err != nil {
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
checkSlug refuses a slug another article's translation into the
language holds, first freeing it from articles in the trash.
*/
func (t *Translations) checkSlug(ctx context.Context, q *models.Queries, a models.Article, locale, slug string) error {
	if err := q.FreeArticleTranslationSlug(ctx, models.FreeArticleTranslationSlugParams{
		Now: t.Clock.Now(), WorkspaceID: a.WorkspaceID, HelpCenterID: a.HelpCenterID, Locale: locale, Slug: slug,
	}); err != nil {
		return fmt.Errorf("free translation slug: %w", err)
	}
	taken, err := q.ArticleTranslationSlugTaken(ctx, models.ArticleTranslationSlugTakenParams{
		WorkspaceID: a.WorkspaceID, HelpCenterID: a.HelpCenterID, Locale: locale, Slug: slug, ArticleID: a.ID,
	})
	if err != nil {
		return fmt.Errorf("check translation slug: %w", err)
	}
	if taken {
		return ErrSlugTaken
	}
	return nil
}

/*
Publish freezes a translation's working copy into a version readers of
its language see, and catches it up with the source.
*/
func (t *Translations) Publish(ctx context.Context, m authz.Membership, articleID ids.UUID, locale string,
) (Translation, error) {
	if !m.Can("articles:write") {
		return Translation{}, exceptions.ErrForbidden
	}
	ws, now := m.Workspace.ID, t.Clock.Now()
	var out Translation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		article, c, err := t.locked(ctx, q, ws, articleID, locale)
		if err != nil {
			return err
		}
		if c.Status == "published" && !c.HasUnpublishedChanges {
			source, err := q.ArticleSourceHash(ctx, models.ArticleSourceHashParams{WorkspaceID: ws, ID: articleID})
			if err != nil {
				return fmt.Errorf("hash source: %w", err)
			}
			if c.SourceHash != nil && *c.SourceHash == source {
				return ErrNothingToPublish
			}
		}
		previous, err := t.freeze(ctx, q, article, c, memberOf(m))
		if err != nil {
			return err
		}
		if previous != "" && previous != c.Slug {
			at := place{workspace: ws, helpCenter: article.HelpCenterID}
			if err := localizedRedirect(ctx, q, at, localized{
				from: "/" + c.Locale + "/articles/" + previous, article: &article.ID, locale: c.Locale, now: now,
			}); err != nil {
				return err
			}
		}
		if err := t.settle(ctx, tx, article); err != nil {
			return err
		}
		out, err = findTranslation(ctx, q, ws, articleID, c.Locale)
		return err
	})
	return out, err
}

/*
freeze renders the translation into a version and points it there,
returning the slug its previous version had, empty when it had none.
*/
func (t *Translations) freeze(ctx context.Context, q *models.Queries, a models.Article, c models.ArticleTranslation,
	member *ids.UUID,
) (string, error) {
	now, previous := t.Clock.Now(), ""
	if c.PublishedVersionID != nil {
		if prev, err := q.FindArticleVersion(ctx, models.FindArticleVersionParams{
			WorkspaceID: c.WorkspaceID, ID: *c.PublishedVersionID,
		}); err == nil {
			previous = prev.Slug
		}
	}
	body, err := render(t.Renderer, c.BodyMarkdown)
	if err != nil {
		return "", err
	}
	number, err := q.NextArticleVersionNumber(ctx, models.NextArticleVersionNumberParams{
		WorkspaceID: c.WorkspaceID, ArticleID: c.ArticleID,
	})
	if err != nil {
		return "", fmt.Errorf("number translation version: %w", err)
	}
	version, err := q.CreateArticleTranslationVersion(ctx, models.CreateArticleTranslationVersionParams{
		ID: ids.NewV7At(now), WorkspaceID: c.WorkspaceID, ArticleID: c.ArticleID, Locale: &c.Locale, Number: number,
		Kind: "published", Title: c.Title, Slug: c.Slug, Description: c.Description, BodyMarkdown: c.BodyMarkdown,
		BodyHtml: body.html, Toc: body.toc, Visibility: a.Visibility, CollectionID: a.CollectionID,
		ContentHash: c.ContentHash, CreatedByType: "member", CreatedByID: member, RendererVersion: RendererVersion,
		Now: now,
	})
	if err != nil {
		return "", fmt.Errorf("freeze translation version: %w", err)
	}
	source, err := q.ArticleSourceHash(ctx, models.ArticleSourceHashParams{WorkspaceID: c.WorkspaceID, ID: c.ArticleID})
	if err != nil {
		return "", fmt.Errorf("hash source: %w", err)
	}
	if _, err := q.PublishArticleTranslation(ctx, models.PublishArticleTranslationParams{
		VersionID: &version.ID, SourceHash: &source, Now: now, WorkspaceID: c.WorkspaceID, ID: c.ID,
	}); err != nil {
		return "", fmt.Errorf("publish translation: %w", err)
	}
	return previous, nil
}

/*
Unpublish takes a translation back to a draft; readers of its language
stop seeing the article.
*/
func (t *Translations) Unpublish(ctx context.Context, m authz.Membership, articleID ids.UUID, locale string,
) (Translation, error) {
	return t.write(ctx, m, articleID, locale, func(ctx context.Context, q *models.Queries, c models.ArticleTranslation,
	) error {
		_, err := q.UnpublishArticleTranslation(ctx, models.UnpublishArticleTranslationParams{
			Now: t.Clock.Now(), WorkspaceID: c.WorkspaceID, ID: c.ID,
		})
		return err
	})
}

/*
MarkCurrent says a translation caught up with the source as it now is,
without publishing it again.
*/
func (t *Translations) MarkCurrent(ctx context.Context, m authz.Membership, articleID ids.UUID, locale string,
) (Translation, error) {
	return t.write(ctx, m, articleID, locale, func(ctx context.Context, q *models.Queries, c models.ArticleTranslation,
	) error {
		source, err := q.ArticleSourceHash(ctx, models.ArticleSourceHashParams{
			WorkspaceID: c.WorkspaceID, ID: c.ArticleID,
		})
		if err != nil {
			return err
		}
		_, err = q.MarkArticleTranslationCurrent(ctx, models.MarkArticleTranslationCurrentParams{
			SourceHash: &source, Now: t.Clock.Now(), WorkspaceID: c.WorkspaceID, ID: c.ID,
		})
		return err
	})
}

/*
Delete removes a translation. Its versions stay in the article's
history.
*/
func (t *Translations) Delete(ctx context.Context, m authz.Membership, articleID ids.UUID, locale string) error {
	if !m.Can("articles:write") {
		return exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		article, c, err := t.locked(ctx, q, ws, articleID, locale)
		if err != nil {
			return err
		}
		if err := q.DeleteArticleTranslation(ctx, models.DeleteArticleTranslationParams{
			WorkspaceID: ws, ID: c.ID,
		}); err != nil {
			return fmt.Errorf("delete translation: %w", err)
		}
		return t.settle(ctx, tx, article)
	})
}

/*
Versions lists the versions of a translation, newest first.
*/
func (t *Translations) Versions(ctx context.Context, m authz.Membership, articleID ids.UUID, locale string,
) ([]models.ListArticleTranslationVersionsRow, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.ListArticleTranslationVersionsRow
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findArticle(ctx, q, m.Workspace.ID, articleID); err != nil {
			return err
		}
		var err error
		out, err = q.ListArticleTranslationVersions(ctx, models.ListArticleTranslationVersionsParams{
			WorkspaceID: m.Workspace.ID, ArticleID: articleID, Locale: new(canonicalLocale(locale)),
		})
		return err
	})
	return out, err
}

/*
Restore puts a version of a translation back into its working copy.
*/
func (t *Translations) Restore(ctx context.Context, m authz.Membership, articleID ids.UUID, locale string,
	version ids.UUID,
) (Translation, error) {
	if !m.Can("articles:write") {
		return Translation{}, exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	var out Translation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		_, c, err := t.locked(ctx, q, ws, articleID, locale)
		if err != nil {
			return err
		}
		kept, err := q.FindArticleVersion(ctx, models.FindArticleVersionParams{WorkspaceID: ws, ID: version})
		if errors.Is(err, database.ErrNotFound) || (err == nil &&
			(kept.ArticleID != articleID || kept.Locale == nil || *kept.Locale != c.Locale)) {
			return exceptions.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find version: %w", err)
		}
		description := deref(kept.Description)
		if err := t.change(ctx, q, c, TranslationInput{
			Membership: m, Title: &kept.Title, Slug: &kept.Slug, Description: &description, Body: &kept.BodyMarkdown,
		}); err != nil {
			return err
		}
		out, err = findTranslation(ctx, q, ws, articleID, c.Locale)
		return err
	})
	return out, err
}

/*
write locks a translation, applies a change readers can see and returns
the translation as it now is.
*/
func (t *Translations) write(ctx context.Context, m authz.Membership, articleID ids.UUID, locale string,
	change func(ctx context.Context, q *models.Queries, c models.ArticleTranslation) error,
) (Translation, error) {
	if !m.Can("articles:write") {
		return Translation{}, exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	var out Translation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		article, c, err := t.locked(ctx, q, ws, articleID, locale)
		if err != nil {
			return err
		}
		if err := change(ctx, q, c); err != nil {
			return fmt.Errorf("change translation: %w", err)
		}
		if err := t.settle(ctx, tx, article); err != nil {
			return err
		}
		out, err = findTranslation(ctx, q, ws, articleID, c.Locale)
		return err
	})
	return out, err
}

/*
locked returns the article and holds its translation into the language
until the transaction ends.
*/
func (t *Translations) locked(ctx context.Context, q *models.Queries, ws, articleID ids.UUID, locale string,
) (models.Article, models.ArticleTranslation, error) {
	article, err := findArticle(ctx, q, ws, articleID)
	if err != nil {
		return article, models.ArticleTranslation{}, err
	}
	c, err := q.LockArticleTranslation(ctx, models.LockArticleTranslationParams{
		WorkspaceID: ws, ArticleID: articleID, Locale: canonicalLocale(locale),
	})
	if errors.Is(err, database.ErrNotFound) {
		return article, c, ErrTranslationNotFound
	}
	if err != nil {
		return article, c, fmt.Errorf("lock translation: %w", err)
	}
	return article, c, nil
}

/*
settle finishes a change readers can see: the knowledge index follows
the article and the help center's cache version moves on.
*/
func (t *Translations) settle(ctx context.Context, tx *database.Tx, a models.Article) error {
	if t.Index != nil {
		if err := t.Index.IndexArticle(ctx, tx, a.WorkspaceID, a.ID); err != nil {
			return err
		}
	}
	return touch(ctx, models.New(tx), place{workspace: a.WorkspaceID, helpCenter: a.HelpCenterID}, t.Clock.Now())
}

/*
TranslationState is where one article stands in one language: missing,
draft or published, and whether it is outdated.
*/
type TranslationState struct {
	ArticleID    ids.UUID
	Title        string
	Status       string
	CollectionID *ids.UUID
	Locale       string
	State        string
	Outdated     bool
}

/*
States returns, for every article of the help center that is not
archived, where it stands in each language the help center offers
besides its default, the articles last changed first.
*/
func (t *Translations) States(ctx context.Context, m authz.Membership, helpCenterID ids.UUID,
) ([]TranslationState, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []TranslationState
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		h, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenterID)
		if err != nil {
			return err
		}
		rows, err := q.ListHelpCenterTranslationStates(ctx, models.ListHelpCenterTranslationStatesParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: h.ID,
		})
		if err != nil {
			return fmt.Errorf("list translation states: %w", err)
		}
		out = statesOf(rows, LocalesOf(h)[1:])
		return nil
	})
	return out, err
}

/*
statesOf turns each article's translation rows into one state per
language offered.
*/
func statesOf(rows []models.ListHelpCenterTranslationStatesRow, locales []string) []TranslationState {
	var out []TranslationState
	for i := 0; i < len(rows); {
		a := rows[i]
		have := map[string]models.ListHelpCenterTranslationStatesRow{}
		for ; i < len(rows) && rows[i].ID == a.ID; i++ {
			if rows[i].Locale != nil {
				have[*rows[i].Locale] = rows[i]
			}
		}
		for _, l := range locales {
			s := TranslationState{
				ArticleID: a.ID, Title: a.Title, Status: a.Status, CollectionID: a.CollectionID, Locale: l,
				State: "missing",
			}
			if r, ok := have[l]; ok {
				s.State, s.Outdated = deref(r.TranslationStatus), r.Outdated
			}
			out = append(out, s)
		}
	}
	return out
}

/*
CollectionTranslationInput is a collection's translation into one
language: its name, and a slug made from it when none is given.
*/
type CollectionTranslationInput struct {
	Membership   authz.Membership
	CollectionID ids.UUID
	Locale       string
	Name         string
	Slug         *string
	Description  *string
}

/*
CollectionTranslation is a collection's translation into one language,
with whether the source changed since it was written.
*/
type CollectionTranslation = models.ListTranslationsOfCollectionRow

/*
ListCollection returns a collection's translations.
*/
func (t *Translations) ListCollection(ctx context.Context, m authz.Membership, collectionID ids.UUID,
) ([]CollectionTranslation, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []CollectionTranslation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findCollection(ctx, q, m.Workspace.ID, collectionID); err != nil {
			return err
		}
		var err error
		out, err = q.ListTranslationsOfCollection(ctx, models.ListTranslationsOfCollectionParams{
			WorkspaceID: m.Workspace.ID, CollectionID: collectionID,
		})
		return err
	})
	return out, err
}

/*
ListForHelpCenter returns every collection translation of a help center.
*/
func (t *Translations) ListForHelpCenter(ctx context.Context, m authz.Membership, helpCenterID ids.UUID,
) ([]CollectionTranslation, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []CollectionTranslation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findHelpCenter(ctx, q, m.Workspace.ID, helpCenterID); err != nil {
			return err
		}
		rows, err := q.ListCollectionTranslations(ctx, models.ListCollectionTranslationsParams{
			WorkspaceID: m.Workspace.ID, HelpCenterID: helpCenterID,
		})
		for _, r := range rows {
			out = append(out, CollectionTranslation(r))
		}
		return err
	})
	return out, err
}

/*
SaveCollection writes a collection's translation into a language. A
slug that changes leaves a redirect in that language.
*/
func (t *Translations) SaveCollection(ctx context.Context, in CollectionTranslationInput,
) (CollectionTranslation, error) {
	if !in.Membership.Can("articles:write") {
		return CollectionTranslation{}, exceptions.ErrForbidden
	}
	ws, now := in.Membership.Workspace.ID, t.Clock.Now()
	var out CollectionTranslation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		col, err := findCollection(ctx, q, ws, in.CollectionID)
		if err != nil {
			return err
		}
		h, err := findHelpCenter(ctx, q, ws, col.HelpCenterID)
		if err != nil {
			return err
		}
		locale, err := checkTranslationLocale(h, in.Locale)
		if err != nil {
			return err
		}
		name := truncate(in.Name, 80)
		if name == "" {
			return fault.New(fault.Invalid, "name_required", "Name the collection.").OnField("name")
		}
		slug := Slugify(name, 80)
		if in.Slug != nil && strings.TrimSpace(*in.Slug) != "" {
			slug = strings.ToLower(strings.TrimSpace(*in.Slug))
		}
		if slug == "" {
			slug = col.Slug
		}
		if len(slug) > 80 || !slugPattern().MatchString(slug) {
			return ErrSlugInvalid
		}
		taken, err := q.CollectionTranslationSlugTaken(ctx, models.CollectionTranslationSlugTakenParams{
			WorkspaceID: ws, HelpCenterID: h.ID, Locale: locale, Slug: slug, CollectionID: col.ID,
		})
		if err != nil {
			return fmt.Errorf("check collection translation slug: %w", err)
		}
		if taken {
			return ErrSlugTaken
		}
		before, err := q.FindCollectionTranslation(ctx, models.FindCollectionTranslationParams{
			WorkspaceID: ws, CollectionID: col.ID, Locale: locale,
		})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("find collection translation: %w", err)
		}
		if _, err := q.UpsertCollectionTranslation(ctx, models.UpsertCollectionTranslationParams{
			Locale: locale, Name: name, Slug: slug, Description: optional(in.Description, 300), Now: now,
			WorkspaceID: ws, CollectionID: col.ID,
		}); err != nil {
			return fmt.Errorf("save collection translation: %w", err)
		}
		at := place{workspace: ws, helpCenter: h.ID}
		if before.Slug != "" && before.Slug != slug {
			if err := localizedRedirect(ctx, q, at, localized{
				from: "/" + locale + "/collections/" + before.Slug, collection: &col.ID, locale: locale, now: now,
			}); err != nil {
				return err
			}
		}
		if err := touch(ctx, q, at, now); err != nil {
			return err
		}
		rows, err := q.ListTranslationsOfCollection(ctx, models.ListTranslationsOfCollectionParams{
			WorkspaceID: ws, CollectionID: col.ID,
		})
		for _, r := range rows {
			if r.Locale == locale {
				out = r
			}
		}
		return err
	})
	return out, err
}

/*
DeleteCollection removes a collection's translation into a language.
*/
func (t *Translations) DeleteCollection(ctx context.Context, m authz.Membership, collectionID ids.UUID,
	locale string,
) error {
	if !m.Can("articles:write") {
		return exceptions.ErrForbidden
	}
	ws := m.Workspace.ID
	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		col, err := findCollection(ctx, q, ws, collectionID)
		if err != nil {
			return err
		}
		locale = canonicalLocale(locale)
		if _, err := q.FindCollectionTranslation(ctx, models.FindCollectionTranslationParams{
			WorkspaceID: ws, CollectionID: col.ID, Locale: locale,
		}); errors.Is(err, database.ErrNotFound) {
			return ErrCollectionTranslationNotFound
		} else if err != nil {
			return fmt.Errorf("find collection translation: %w", err)
		}
		if err := q.DeleteCollectionTranslation(ctx, models.DeleteCollectionTranslationParams{
			WorkspaceID: ws, CollectionID: col.ID, Locale: locale,
		}); err != nil {
			return fmt.Errorf("delete collection translation: %w", err)
		}
		return touch(ctx, q, place{workspace: ws, helpCenter: col.HelpCenterID}, t.Clock.Now())
	})
}

/*
localized is an address of one language that changed: the old path and
the article or collection it now leads to in that language.
*/
type localized struct {
	from       string
	article    *ids.UUID
	collection *ids.UUID
	locale     string
	now        time.Time
}

/*
localizedRedirect makes an old path of one language answer with its
target in that language.
*/
func localizedRedirect(ctx context.Context, q *models.Queries, at place, l localized) error {
	kind := "article"
	if l.collection != nil {
		kind = "collection"
	}
	err := q.UpsertLocalizedRedirect(ctx, models.UpsertLocalizedRedirectParams{
		WorkspaceID: at.workspace, HelpCenterID: at.helpCenter, FromPath: l.from, TargetType: kind,
		TargetArticleID: l.article, TargetCollectionID: l.collection, TargetLocale: &l.locale,
		Source: "slug_change", Now: l.now,
	})
	if err != nil {
		return fmt.Errorf("record redirect: %w", err)
	}
	return nil
}

/*
articleAndCenter returns a live article and its help center.
*/
func articleAndCenter(ctx context.Context, q *models.Queries, ws, id ids.UUID,
) (models.Article, models.HelpCenter, error) {
	a, err := findArticle(ctx, q, ws, id)
	if err != nil {
		return a, models.HelpCenter{}, err
	}
	h, err := findHelpCenter(ctx, q, ws, a.HelpCenterID)
	return a, h, err
}

/*
findTranslation returns an article's translation into a language, or
404.
*/
func findTranslation(ctx context.Context, q *models.Queries, ws, articleID ids.UUID, locale string,
) (Translation, error) {
	out, err := q.FindArticleTranslation(ctx, models.FindArticleTranslationParams{
		WorkspaceID: ws, ArticleID: articleID, Locale: locale,
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrTranslationNotFound
	}
	if err != nil {
		return out, fmt.Errorf("find translation: %w", err)
	}
	return out, nil
}

/*
translationPublishedHash is the content hash of a translation's live
version, empty when it has none.
*/
func translationPublishedHash(ctx context.Context, q *models.Queries, c models.ArticleTranslation) (string, error) {
	if c.PublishedVersionID == nil {
		return "", nil
	}
	v, err := q.FindArticleVersion(ctx, models.FindArticleVersionParams{
		WorkspaceID: c.WorkspaceID, ID: *c.PublishedVersionID,
	})
	if err != nil {
		return "", fmt.Errorf("find published translation version: %w", err)
	}
	return v.ContentHash, nil
}

/*
canonicalLocale writes a language tag as it is stored, or leaves it as
given when it is no tag, so a lookup simply finds nothing.
*/
func canonicalLocale(raw string) string {
	if l, ok := NormalizeLocale(raw); ok {
		return l
	}
	return raw
}

/*
textHash is the content hash of a translation's words.
*/
func (d *draft) textHash() string {
	return hashing.SHA256Hex([]byte(strings.Join([]string{d.title, d.slug, d.description, d.body}, "\x00")))
}
