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
asked and it changed. It returns the language the translation went to,
empty when the document has no place in the help center.
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
	if err := a.turnOn(ctx, q, h, locale); err != nil {
		return "", err
	}
	t := &Translations{Clock: a.Clock, Renderer: a.Renderer, Index: a.Index}
	if err := t.importWords(ctx, q, article, importedWords(in, article, locale)); err != nil || !in.Publish {
		return locale, err
	}
	published, err := t.importPublish(ctx, q, article, locale)
	if err != nil || !published {
		return locale, err
	}
	return locale, t.settle(ctx, tx, article)
}

/*
turnOn offers the language on the help center when it did not yet.
*/
func (a *Articles) turnOn(ctx context.Context, q *models.Queries, h models.HelpCenter, locale string) error {
	if OffersLocale(h, locale) {
		return nil
	}
	locales := append(LocalesOf(h), locale)
	_, err := setLanguages(ctx, q, h, LanguagesChange{Locales: &locales, now: a.Clock.Now})
	return err
}

/*
importedWords is the document as a translation to write: its title, or
the article's when it has none, and its body cut to the largest one.
*/
func importedWords(in ImportTranslationInput, article models.Article, locale string) TranslationInput {
	title := truncate(in.Title, 150)
	if title == "" {
		title = article.Title
	}
	body := in.Body
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	return TranslationInput{
		Membership: authz.Membership{Workspace: models.Workspace{ID: in.WorkspaceID}}, ArticleID: article.ID,
		Locale: locale, Title: &title, Body: &body,
	}
}

/*
importWords writes the translation, keeping the slug an earlier import
gave it.
*/
func (t *Translations) importWords(ctx context.Context, q *models.Queries, article models.Article,
	write TranslationInput,
) error {
	current, err := q.LockArticleTranslation(ctx, models.LockArticleTranslationParams{
		WorkspaceID: article.WorkspaceID, ArticleID: article.ID, Locale: write.Locale,
	})
	switch {
	case errors.Is(err, database.ErrNotFound):
		return t.importNew(ctx, q, article, write)
	case err != nil:
		return fmt.Errorf("lock translation: %w", err)
	}
	write.Slug = &current.Slug
	return t.change(ctx, q, current, write)
}

/*
importPublish publishes the imported translation unless it is live and
unchanged, and says whether it did.
*/
func (t *Translations) importPublish(ctx context.Context, q *models.Queries, article models.Article, locale string,
) (bool, error) {
	current, err := q.LockArticleTranslation(ctx, models.LockArticleTranslationParams{
		WorkspaceID: article.WorkspaceID, ArticleID: article.ID, Locale: locale,
	})
	if err != nil {
		return false, fmt.Errorf("lock translation: %w", err)
	}
	if current.Status == "published" && !current.HasUnpublishedChanges {
		return false, nil
	}
	_, err = t.freeze(ctx, q, heldTranslation{article: article, translation: current})
	return err == nil, err
}

/*
importNew makes the translation with a free slug, numbered when another
article's translation into the language holds it.
*/
func (t *Translations) importNew(ctx context.Context, q *models.Queries, a models.Article, in TranslationInput) error {
	base := translatedSlug(*in.Title, a.Slug, 110)
	for n := 1; n < 100; n++ {
		slug := base
		if n > 1 {
			slug = fmt.Sprintf("%s-%d", truncate(base, 110), n)
		}
		in.Slug = &slug
		err := t.create(ctx, q, a, in)
		if !errors.Is(err, ErrSlugTaken) {
			return err
		}
	}
	return ErrSlugTaken
}

/*
ImportedTranslation is an imported translation's old address: the
article, the language and the path it had in the old help center.
*/
type ImportedTranslation struct {
	Article  models.Article
	Locale   string
	FromPath string
}

/*
ImportTranslationRedirect makes an address of the old help center lead
to the imported translation for good.
*/
func (a *Articles) ImportTranslationRedirect(ctx context.Context, tx *database.Tx, in ImportedTranslation) error {
	from, art := in.FromPath, in.Article
	if from == "" || from == "/" || len(from) > 512 || in.Locale == "" {
		return nil
	}
	err := models.New(tx).UpsertLocalizedRedirect(ctx, models.UpsertLocalizedRedirectParams{
		WorkspaceID: art.WorkspaceID, HelpCenterID: art.HelpCenterID, FromPath: from, TargetType: "article",
		TargetArticleID: &art.ID, TargetLocale: &in.Locale, Source: "import", Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("record import redirect: %w", err)
	}
	return nil
}
