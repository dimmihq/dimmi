/*
|--------------------------------------------------------------------------
| Collection Translations
|--------------------------------------------------------------------------
|
| A collection's name, slug and description in another language the
| help center speaks. Writing one catches it up with the collection's
| own name as it now is; a slug that changes leaves a redirect in that
| language. Readers of the language see the collection once it holds an
| article they can read.
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
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

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
	var out CollectionTranslation
	err := t.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		w, err := t.collectionWrite(ctx, q, in)
		if err != nil {
			return err
		}
		if err := t.storeCollection(ctx, q, w); err != nil {
			return err
		}
		rows, err := q.ListTranslationsOfCollection(ctx, models.ListTranslationsOfCollectionParams{
			WorkspaceID: w.col.WorkspaceID, CollectionID: w.col.ID,
		})
		for _, r := range rows {
			if r.Locale == w.locale {
				out = r
			}
		}
		return err
	})
	return out, err
}

/*
collectionWrite is a checked write of a collection's translation: the
collection, the language, the name and slug, and the translation it
replaces, empty for a new one.
*/
type collectionWrite struct {
	col         models.Collection
	locale      string
	name        string
	slug        string
	description *string
	before      models.CollectionTranslation
}

/*
collectionWrite checks a collection's translation before it is stored.
*/
func (t *Translations) collectionWrite(ctx context.Context, q *models.Queries, in CollectionTranslationInput,
) (collectionWrite, error) {
	ws := in.Membership.Workspace.ID
	w := collectionWrite{name: truncate(in.Name, 80), description: optional(in.Description, 300)}
	var err error
	if w.col, err = findCollection(ctx, q, ws, in.CollectionID); err != nil {
		return w, err
	}
	h, err := findHelpCenter(ctx, q, ws, w.col.HelpCenterID)
	if err != nil {
		return w, err
	}
	if w.locale, err = checkTranslationLocale(h, in.Locale); err != nil {
		return w, err
	}
	if w.name == "" {
		return w, fault.New(fault.Invalid, "name_required", "Name the collection.").OnField("name")
	}
	w.before, err = q.FindCollectionTranslation(ctx, models.FindCollectionTranslationParams{
		WorkspaceID: ws, CollectionID: w.col.ID, Locale: w.locale,
	})
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return w, fmt.Errorf("find collection translation: %w", err)
	}
	w.slug = collectionSlugOf(w, in.Slug)
	return w, checkCollectionSlug(ctx, q, w)
}

/*
collectionSlugOf is the slug a write takes: the one asked for, else the
one the translation has, else one made from its name.
*/
func collectionSlugOf(w collectionWrite, asked *string) string {
	if asked != nil && strings.TrimSpace(*asked) != "" {
		return strings.ToLower(strings.TrimSpace(*asked))
	}
	if w.before.Slug != "" {
		return w.before.Slug
	}
	return translatedSlug(w.name, w.col.Slug, 80)
}

/*
checkCollectionSlug refuses a malformed slug and one another
collection's translation into the language holds.
*/
func checkCollectionSlug(ctx context.Context, q *models.Queries, w collectionWrite) error {
	if len(w.slug) > 80 || !slugPattern().MatchString(w.slug) {
		return ErrSlugInvalid
	}
	taken, err := q.CollectionTranslationSlugTaken(ctx, models.CollectionTranslationSlugTakenParams{
		WorkspaceID: w.col.WorkspaceID, HelpCenterID: w.col.HelpCenterID, Locale: w.locale, Slug: w.slug,
		CollectionID: w.col.ID,
	})
	if err != nil {
		return fmt.Errorf("check collection translation slug: %w", err)
	}
	if taken {
		return ErrSlugTaken
	}
	return nil
}

/*
storeCollection writes the translation, leaves a redirect from a slug
it gave up, and moves the help center's cache on.
*/
func (t *Translations) storeCollection(ctx context.Context, q *models.Queries, w collectionWrite) error {
	now := t.Clock.Now()
	if _, err := q.UpsertCollectionTranslation(ctx, models.UpsertCollectionTranslationParams{
		Locale: w.locale, Name: w.name, Slug: w.slug, Description: w.description, Now: now,
		WorkspaceID: w.col.WorkspaceID, CollectionID: w.col.ID,
	}); err != nil {
		return fmt.Errorf("save collection translation: %w", err)
	}
	at := place{workspace: w.col.WorkspaceID, helpCenter: w.col.HelpCenterID}
	if w.before.Slug != "" && w.before.Slug != w.slug {
		if err := localizedRedirect(ctx, q, at, localized{
			from: "/" + w.locale + "/collections/" + w.before.Slug, collection: &w.col.ID, locale: w.locale, now: now,
		}); err != nil {
			return err
		}
	}
	return touch(ctx, q, at, now)
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
