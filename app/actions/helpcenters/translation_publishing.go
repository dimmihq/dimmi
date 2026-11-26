/*
|--------------------------------------------------------------------------
| Publishing Translations
|--------------------------------------------------------------------------
|
| A translation goes live in its language on its own: publishing freezes
| its working copy into a version, numbered in the same run as the
| article's other versions, and catches it up with the source. A
| published slug that changes leaves a redirect in its language.
| Unpublishing, marking up to date, deleting and restoring a version
| each touch one translation, and every change readers can see moves the
| knowledge index and the help center's cache on.
|
*/

package helpcenters

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Publish freezes a translation's working copy into a version readers of
its language see, and catches it up with the source.
*/
func (t *Translations) Publish(ctx context.Context, m authz.Membership, ref TranslationRef) (Translation, error) {
	return t.apply(ctx, m, ref, func(ctx context.Context, q *models.Queries, held heldTranslation) error {
		if err := nothingNew(ctx, q, held.translation); err != nil {
			return err
		}
		previous, err := t.freeze(ctx, q, held)
		if err != nil {
			return err
		}
		return t.leaveRedirect(ctx, q, held, previous)
	})
}

/*
nothingNew refuses to publish a translation that is live, unchanged and
caught up with the source.
*/
func nothingNew(ctx context.Context, q *models.Queries, c models.ArticleTranslation) error {
	if c.Status != "published" || c.HasUnpublishedChanges {
		return nil
	}
	source, err := q.ArticleSourceHash(ctx, models.ArticleSourceHashParams{WorkspaceID: c.WorkspaceID, ID: c.ArticleID})
	if err != nil {
		return fmt.Errorf("hash source: %w", err)
	}
	if c.SourceHash != nil && *c.SourceHash == source {
		return ErrNothingToPublish
	}
	return nil
}

/*
leaveRedirect makes the address a translation had before a publish lead
to its new one.
*/
func (t *Translations) leaveRedirect(ctx context.Context, q *models.Queries, held heldTranslation, previous string,
) error {
	c, a := held.translation, held.article
	if previous == "" || previous == c.Slug {
		return nil
	}
	return localizedRedirect(ctx, q, place{workspace: a.WorkspaceID, helpCenter: a.HelpCenterID}, localized{
		from: "/" + c.Locale + "/articles/" + previous, article: &a.ID, locale: c.Locale, now: t.Clock.Now(),
	})
}

/*
heldTranslation is a translation held until the transaction ends, with
its article and who acts on it.
*/
type heldTranslation struct {
	article     models.Article
	translation models.ArticleTranslation
	member      *ids.UUID
}

/*
freeze renders the translation into a version and points it there,
returning the slug its previous version had, empty when it had none.
*/
func (t *Translations) freeze(ctx context.Context, q *models.Queries, held heldTranslation) (string, error) {
	a, c := held.article, held.translation
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
		ContentHash: c.ContentHash, CreatedByType: "member", CreatedByID: held.member,
		RendererVersion: RendererVersion, Now: now,
	})
	if err != nil {
		return "", fmt.Errorf("freeze translation version: %w", err)
	}
	return previous, t.pointAt(ctx, q, c, version.ID)
}

/*
pointAt makes the version the translation's live one, caught up with
the source as it now is.
*/
func (t *Translations) pointAt(ctx context.Context, q *models.Queries, c models.ArticleTranslation, version ids.UUID,
) error {
	source, err := q.ArticleSourceHash(ctx, models.ArticleSourceHashParams{WorkspaceID: c.WorkspaceID, ID: c.ArticleID})
	if err != nil {
		return fmt.Errorf("hash source: %w", err)
	}
	if _, err := q.PublishArticleTranslation(ctx, models.PublishArticleTranslationParams{
		VersionID: &version, SourceHash: &source, Now: t.Clock.Now(), WorkspaceID: c.WorkspaceID, ID: c.ID,
	}); err != nil {
		return fmt.Errorf("publish translation: %w", err)
	}
	return nil
}

/*
Unpublish takes a translation back to a draft; readers of its language
stop seeing it.
*/
func (t *Translations) Unpublish(ctx context.Context, m authz.Membership, ref TranslationRef) (Translation, error) {
	return t.apply(ctx, m, ref, func(ctx context.Context, q *models.Queries, held heldTranslation) error {
		c := held.translation
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
func (t *Translations) MarkCurrent(ctx context.Context, m authz.Membership, ref TranslationRef) (Translation, error) {
	return t.apply(ctx, m, ref, func(ctx context.Context, q *models.Queries, held heldTranslation) error {
		c := held.translation
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
func (t *Translations) Delete(ctx context.Context, m authz.Membership, ref TranslationRef) error {
	if !m.Can("articles:write") {
		return exceptions.ErrForbidden
	}
	return t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		held, err := t.hold(ctx, q, m, ref)
		if err != nil {
			return err
		}
		if err := q.DeleteArticleTranslation(ctx, models.DeleteArticleTranslationParams{
			WorkspaceID: m.Workspace.ID, ID: held.translation.ID,
		}); err != nil {
			return fmt.Errorf("delete translation: %w", err)
		}
		return t.settle(ctx, tx, held.article)
	})
}

/*
Versions lists the versions of a translation, newest first.
*/
func (t *Translations) Versions(ctx context.Context, m authz.Membership, ref TranslationRef,
) ([]models.ListArticleTranslationVersionsRow, error) {
	if !m.Can("articles:read") {
		return nil, exceptions.ErrForbidden
	}
	var out []models.ListArticleTranslationVersionsRow
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findArticle(ctx, q, m.Workspace.ID, ref.Article); err != nil {
			return err
		}
		var err error
		out, err = q.ListArticleTranslationVersions(ctx, models.ListArticleTranslationVersionsParams{
			WorkspaceID: m.Workspace.ID, ArticleID: ref.Article, Locale: new(canonicalLocale(ref.Locale)),
		})
		return err
	})
	return out, err
}

/*
Restore puts a version of a translation back into its working copy.
*/
func (t *Translations) Restore(ctx context.Context, m authz.Membership, ref TranslationRef, version ids.UUID,
) (Translation, error) {
	return t.apply(ctx, m, ref, func(ctx context.Context, q *models.Queries, held heldTranslation) error {
		c := held.translation
		kept, err := q.FindArticleVersion(ctx, models.FindArticleVersionParams{WorkspaceID: c.WorkspaceID, ID: version})
		if errors.Is(err, database.ErrNotFound) || (err == nil &&
			(kept.ArticleID != c.ArticleID || kept.Locale == nil || *kept.Locale != c.Locale)) {
			return exceptions.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find version: %w", err)
		}
		description := deref(kept.Description)
		return t.change(ctx, q, c, TranslationInput{
			Membership: m, Title: &kept.Title, Slug: &kept.Slug, Description: &description, Body: &kept.BodyMarkdown,
		})
	})
}

/*
apply holds a translation, applies a change and returns the translation
as it now is, moving the index and cache on.
*/
func (t *Translations) apply(ctx context.Context, m authz.Membership, ref TranslationRef,
	change func(ctx context.Context, q *models.Queries, held heldTranslation) error,
) (Translation, error) {
	if !m.Can("articles:write") {
		return Translation{}, exceptions.ErrForbidden
	}
	var out Translation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		held, err := t.hold(ctx, q, m, ref)
		if err != nil {
			return err
		}
		if err := change(ctx, q, held); err != nil {
			return err
		}
		if err := t.settle(ctx, tx, held.article); err != nil {
			return err
		}
		ref.Locale = held.translation.Locale
		out, err = findTranslation(ctx, q, m.Workspace.ID, ref)
		return err
	})
	return out, err
}

/*
hold returns the article and holds its translation into the language
until the transaction ends.
*/
func (t *Translations) hold(ctx context.Context, q *models.Queries, m authz.Membership, ref TranslationRef,
) (heldTranslation, error) {
	out := heldTranslation{member: memberOf(m)}
	var err error
	if out.article, err = findArticle(ctx, q, m.Workspace.ID, ref.Article); err != nil {
		return out, err
	}
	out.translation, err = q.LockArticleTranslation(ctx, models.LockArticleTranslationParams{
		WorkspaceID: m.Workspace.ID, ArticleID: ref.Article, Locale: canonicalLocale(ref.Locale),
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrTranslationNotFound
	}
	if err != nil {
		return out, fmt.Errorf("lock translation: %w", err)
	}
	return out, nil
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
