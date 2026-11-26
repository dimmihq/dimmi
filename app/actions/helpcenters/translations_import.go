/*
|--------------------------------------------------------------------------
| Importing Translations
|--------------------------------------------------------------------------
|
| A help center brought over from another tool keeps all its languages
| in one help center: a document translating another becomes that
| article's translation, and the language is turned on for the help
| center when it was not yet. A language dimmi offers no prefix for is
| read as its base language, fr-FR as fr, and one with neither is left
| out. An import updates the translation an earlier one made, and
| publishes it when the document is published and its text changed.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ImportTranslationInput is a document translating an imported article.
*/
type ImportTranslationInput struct {
	WorkspaceID ids.UUID
	ArticleID   ids.UUID
	Locale      string
	Title       string
	Body        string
	Publish     bool
}

/*
ImportLocale is the language a help center would offer an imported
document's language as: the default when it is written in it, else a
language dimmi offers, the base one when the region is not offered,
false when there is none.
*/
func ImportLocale(h models.HelpCenter, raw string) (string, bool) {
	tag, ok := NormalizeLocale(raw)
	if !ok {
		return "", false
	}
	base, _, _ := strings.Cut(tag, "-")
	defBase, _, _ := strings.Cut(h.Locale, "-")
	switch {
	case tag == h.Locale || base == defBase:
		return h.Locale, true
	case OffersLocale(h, tag) || Listed(tag):
		return tag, true
	case OffersLocale(h, base) || Listed(base):
		return base, true
	}
	return "", false
}

/*
ImportTranslation writes the document as the article's translation,
turning its language on for the help center, and publishes it when
asked and it changed. It returns the language the translation went to.
*/
func (a *Articles) ImportTranslation(ctx context.Context, tx *database.Tx, in ImportTranslationInput,
) (string, error) {
	q := models.New(tx)
	article, h, err := articleAndCenter(ctx, q, in.WorkspaceID, in.ArticleID)
	if err != nil {
		return "", err
	}
	locale, ok := ImportLocale(h, in.Locale)
	if !ok || locale == h.Locale {
		return "", nil
	}
	if !OffersLocale(h, locale) {
		locales := append(LocalesOf(h), locale)
		if _, err := setLanguages(ctx, q, h, LanguagesChange{Locales: &locales}, a.Clock.Now); err != nil {
			return "", err
		}
	}
	t := &Translations{Clock: a.Clock, Renderer: a.Renderer, Index: a.Index}
	title := truncate(in.Title, 150)
	if title == "" {
		title = article.Title
	}
	body := in.Body
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	write := TranslationInput{
		Membership: authz.Membership{Workspace: models.Workspace{ID: in.WorkspaceID}}, ArticleID: article.ID,
		Locale: locale, Title: &title, Body: &body,
	}
	current, err := q.LockArticleTranslation(ctx, models.LockArticleTranslationParams{
		WorkspaceID: in.WorkspaceID, ArticleID: article.ID, Locale: locale,
	})
	switch {
	case errors.Is(err, database.ErrNotFound):
		err = t.importNew(ctx, q, article, write)
	case err != nil:
		return "", fmt.Errorf("lock translation: %w", err)
	default:
		write.Slug = &current.Slug
		err = t.change(ctx, q, current, write)
	}
	if err != nil || !in.Publish {
		return locale, err
	}
	current, err = q.LockArticleTranslation(ctx, models.LockArticleTranslationParams{
		WorkspaceID: in.WorkspaceID, ArticleID: article.ID, Locale: locale,
	})
	if err != nil {
		return locale, fmt.Errorf("lock translation: %w", err)
	}
	if current.Status == "published" && !current.HasUnpublishedChanges {
		return locale, nil
	}
	if _, err := t.freeze(ctx, q, article, current, nil); err != nil {
		return locale, err
	}
	return locale, t.settle(ctx, tx, article)
}

/*
importNew makes the translation with a free slug, numbered when another
article's translation into the language holds it.
*/
func (t *Translations) importNew(ctx context.Context, q *models.Queries, a models.Article, in TranslationInput) error {
	base := Slugify(*in.Title, 110)
	if base == "" {
		base = a.Slug
	}
	for n := 1; n < 100; n++ {
		slug := base
		if n > 1 {
			slug = fmt.Sprintf("%s-%d", truncate(base, 110), n)
		}
		in.Slug = &slug
		err := t.create(ctx, q, a, in.Locale, in)
		if !errors.Is(err, ErrSlugTaken) {
			return err
		}
	}
	return ErrSlugTaken
}

/*
ImportTranslationRedirect makes an address of the old help center lead
to the imported translation for good.
*/
func (a *Articles) ImportTranslationRedirect(ctx context.Context, tx *database.Tx, art models.Article, locale,
	fromPath string,
) error {
	if fromPath == "" || fromPath == "/" || len(fromPath) > 512 || locale == "" {
		return nil
	}
	err := models.New(tx).UpsertLocalizedRedirect(ctx, models.UpsertLocalizedRedirectParams{
		WorkspaceID: art.WorkspaceID, HelpCenterID: art.HelpCenterID, FromPath: fromPath, TargetType: "article",
		TargetArticleID: &art.ID, TargetLocale: &locale, Source: "import", Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("record import redirect: %w", err)
	}
	return nil
}
