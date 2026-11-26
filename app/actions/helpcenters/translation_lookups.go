/*
|--------------------------------------------------------------------------
| Translation Lookups
|--------------------------------------------------------------------------
|
| What the translation actions share: finding a translation and its
| article, the slug a new one starts with, the hash of its words, and
| the redirect an address of one language leaves when it moves.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

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
func findTranslation(ctx context.Context, q *models.Queries, ws ids.UUID, ref TranslationRef) (Translation, error) {
	out, err := q.FindArticleTranslation(ctx, models.FindArticleTranslationParams{
		WorkspaceID: ws, ArticleID: ref.Article, Locale: canonicalLocale(ref.Locale),
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

/*
translatedSlug is the slug a new translation starts with: one made from
its title in a Latin script, else the source's, since a title in
Arabic, Japanese or Russian would only spell itself out in Latin letters
readers do not recognize.
*/
func translatedSlug(title, source string, limit int) string {
	for _, r := range title {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
			return source
		}
	}
	if slug := Slugify(title, limit); slug != "" {
		return slug
	}
	return source
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
