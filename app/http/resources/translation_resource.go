/*
|--------------------------------------------------------------------------
| Translation Resources
|--------------------------------------------------------------------------
|
| How translations read over the API. An article's translation is its
| text in one language, with where it stands: draft or published,
| published with changes not yet live, and outdated when the source
| changed since it caught up. A collection's translation is its name,
| slug and description in one language.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ArticleTranslation is an article's translation into one language.
*/
type ArticleTranslation struct {
	Object                string     `json:"object"`
	ArticleID             string     `json:"article_id"`
	Locale                string     `json:"locale"`
	Title                 string     `json:"title"`
	Slug                  string     `json:"slug"`
	Description           *string    `json:"description"`
	BodyMarkdown          *string    `json:"body_markdown,omitempty"`
	Status                string     `json:"status"`
	HasUnpublishedChanges bool       `json:"has_unpublished_changes"`
	Outdated              bool       `json:"outdated"`
	LockVersion           int32      `json:"lock_version"`
	WordCount             int32      `json:"word_count"`
	PublishedAt           *time.Time `json:"published_at"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

/*
NewArticleTranslation shapes one translation with its body.
*/
func NewArticleTranslation(t helpcenters.Translation) ArticleTranslation {
	out := translationSummary(t)
	out.BodyMarkdown = &t.BodyMarkdown
	return out
}

/*
NewArticleTranslations shapes a list of translations without bodies.
*/
func NewArticleTranslations(rows []helpcenters.Translation) []ArticleTranslation {
	out := make([]ArticleTranslation, len(rows))
	for i, r := range rows {
		out[i] = translationSummary(r)
	}
	return out
}

func translationSummary(t helpcenters.Translation) ArticleTranslation {
	return ArticleTranslation{
		Object: "article_translation", ArticleID: models.PublicID(models.PrefixArticle, t.ArticleID), Locale: t.Locale,
		Title: t.Title, Slug: t.Slug, Description: t.Description, Status: t.Status,
		HasUnpublishedChanges: t.HasUnpublishedChanges && t.Status == "published", Outdated: t.Outdated,
		LockVersion: t.LockVersion, WordCount: t.WordCount, PublishedAt: t.PublishedAt, CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

/*
NewTranslationVersions shapes the versions of a translation.
*/
func NewTranslationVersions(rows []models.ListArticleTranslationVersionsRow) []ArticleVersion {
	out := make([]ArticleVersion, len(rows))
	for i, r := range rows {
		out[i] = ArticleVersion{
			ID: models.PublicID(models.PrefixArticleVersion, r.ID), Object: "article_version", Number: r.Number,
			Kind: r.Kind, Title: r.Title, CreatedAt: r.CreatedAt,
		}
	}
	return out
}

/*
TranslationState is where one article stands in one language: missing,
draft or published, and whether it is outdated.
*/
type TranslationState struct {
	Object       string  `json:"object"`
	ArticleID    string  `json:"article_id"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	CollectionID *string `json:"collection_id"`
	Locale       string  `json:"locale"`
	State        string  `json:"state"`
	Outdated     bool    `json:"outdated"`
}

/*
NewTranslationStates shapes where articles stand in each language.
*/
func NewTranslationStates(rows []helpcenters.TranslationState) []TranslationState {
	out := make([]TranslationState, len(rows))
	for i, r := range rows {
		out[i] = TranslationState{
			Object: "translation_state", ArticleID: models.PublicID(models.PrefixArticle, r.ArticleID), Title: r.Title,
			Status: r.Status, Locale: r.Locale, State: r.State, Outdated: r.Outdated,
		}
		if r.CollectionID != nil {
			out[i].CollectionID = new(models.PublicID(models.PrefixCollection, *r.CollectionID))
		}
	}
	return out
}

/*
CollectionTranslation is a collection's name, slug and description in
one language.
*/
type CollectionTranslation struct {
	Object       string    `json:"object"`
	CollectionID string    `json:"collection_id"`
	Locale       string    `json:"locale"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  *string   `json:"description"`
	Outdated     bool      `json:"outdated"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

/*
NewCollectionTranslation shapes one collection translation.
*/
func NewCollectionTranslation(t helpcenters.CollectionTranslation) CollectionTranslation {
	return CollectionTranslation{
		Object: "collection_translation", CollectionID: models.PublicID(models.PrefixCollection, t.CollectionID),
		Locale: t.Locale, Name: t.Name, Slug: t.Slug, Description: t.Description, Outdated: t.Outdated,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

/*
NewCollectionTranslations shapes a list of collection translations.
*/
func NewCollectionTranslations(rows []helpcenters.CollectionTranslation) []CollectionTranslation {
	out := make([]CollectionTranslation, len(rows))
	for i, r := range rows {
		out[i] = NewCollectionTranslation(r)
	}
	return out
}

/*
Language is a language a help center may offer.
*/
type Language struct {
	Object string `json:"object"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Native string `json:"native_name"`
}

/*
NewLanguages shapes the languages the editor offers.
*/
func NewLanguages(rows []helpcenters.Language) []Language {
	out := make([]Language, len(rows))
	for i, r := range rows {
		out[i] = Language{Object: "language", Code: r.Code, Name: r.Name, Native: r.Native}
	}
	return out
}
