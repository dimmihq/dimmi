/*
|--------------------------------------------------------------------------
| Translation Controller
|--------------------------------------------------------------------------
|
| An article's translations into the languages its help center offers:
| listed, read, saved, published, unpublished, marked up to date with
| the source, deleted, and their version history. A help center's list
| of where every article stands in each language shows what is missing
| and what is outdated. Collections get their translated name, slug and
| description the same way.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TranslationController serves article and collection translations.
*/
type TranslationController struct {
	Translations *helpcenters.Translations
	IDs          *ids.Registry
}

/*
ArticleTranslationsOutput is a list of an article's translations.
*/
type ArticleTranslationsOutput struct {
	Body fhttp.Collection[resources.ArticleTranslation]
}

/*
ArticleTranslationOutput is one translation with its body.
*/
type ArticleTranslationOutput struct {
	Body fhttp.Resource[resources.ArticleTranslation]
}

/*
TranslationStatesOutput is where a help center's articles stand in each
language.
*/
type TranslationStatesOutput struct {
	Body fhttp.Collection[resources.TranslationState]
}

/*
CollectionTranslationsOutput is a list of collection translations.
*/
type CollectionTranslationsOutput struct {
	Body fhttp.Collection[resources.CollectionTranslation]
}

/*
CollectionTranslationOutput is one collection translation.
*/
type CollectionTranslationOutput struct {
	Body fhttp.Resource[resources.CollectionTranslation]
}

/*
LanguagesOutput is the languages a help center may offer.
*/
type LanguagesOutput struct {
	Body fhttp.Collection[resources.Language]
}

/*
Index lists an article's translations.
*/
func (c *TranslationController) Index(ctx context.Context, in *requests.ArticleRequest,
) (*ArticleTranslationsOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	rows, err := c.Translations.List(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	items := resources.NewArticleTranslations(rows)
	page := pagination.Page[resources.ArticleTranslation]{Items: items, Limit: helpcenters.MaxLocales}
	return &ArticleTranslationsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Show answers one translation with its body.
*/
func (c *TranslationController) Show(ctx context.Context, in *requests.ArticleTranslationRequest,
) (*ArticleTranslationOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	t, err := c.Translations.Show(ctx, membership(ctx), id, in.Locale)
	return translationOutput(t, err)
}

/*
Update saves a translation's working copy, starting a draft when there
is none.
*/
func (c *TranslationController) Update(ctx context.Context, in *requests.UpdateArticleTranslationRequest,
) (*ArticleTranslationOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	b := in.Body
	t, err := c.Translations.Save(ctx, helpcenters.TranslationInput{
		Membership: membership(ctx), ArticleID: id, Locale: in.Locale, Title: b.Title, Slug: b.Slug,
		Description: b.Description, Body: b.BodyMarkdown, LockVersion: b.LockVersion,
	})
	return translationOutput(t, err)
}

/*
Destroy deletes a translation; its versions stay in the history.
*/
func (c *TranslationController) Destroy(ctx context.Context, in *requests.ArticleTranslationRequest,
) (*struct{}, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Translations.Delete(ctx, membership(ctx), id, in.Locale)
}

/*
Publish makes a translation's working copy live in its language.
*/
func (c *TranslationController) Publish(ctx context.Context, in *requests.ArticleTranslationRequest,
) (*ArticleTranslationOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	t, err := c.Translations.Publish(ctx, membership(ctx), id, in.Locale)
	return translationOutput(t, err)
}

/*
Unpublish takes a translation back to a draft.
*/
func (c *TranslationController) Unpublish(ctx context.Context, in *requests.ArticleTranslationRequest,
) (*ArticleTranslationOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	t, err := c.Translations.Unpublish(ctx, membership(ctx), id, in.Locale)
	return translationOutput(t, err)
}

/*
Review marks a translation up to date with the source as it now is.
*/
func (c *TranslationController) Review(ctx context.Context, in *requests.ArticleTranslationRequest,
) (*ArticleTranslationOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	t, err := c.Translations.MarkCurrent(ctx, membership(ctx), id, in.Locale)
	return translationOutput(t, err)
}

/*
Versions lists a translation's versions.
*/
func (c *TranslationController) Versions(ctx context.Context, in *requests.ArticleTranslationRequest,
) (*ArticleVersionsOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	rows, err := c.Translations.Versions(ctx, membership(ctx), id, in.Locale)
	if err != nil {
		return nil, err
	}
	page := pagination.Page[resources.ArticleVersion]{Items: resources.NewTranslationVersions(rows), Limit: 1000}
	return &ArticleVersionsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Restore puts a version of the translation back into its working copy.
*/
func (c *TranslationController) Restore(ctx context.Context, in *requests.RestoreArticleTranslationRequest,
) (*ArticleTranslationOutput, error) {
	id, err := c.decode(models.PrefixArticle, in.Article)
	if err != nil {
		return nil, err
	}
	version, err := c.decode(models.PrefixArticleVersion, in.Body.VersionID)
	if err != nil {
		return nil, err
	}
	t, err := c.Translations.Restore(ctx, membership(ctx), id, in.Locale, version)
	return translationOutput(t, err)
}

/*
States lists where every article of a help center stands in each
language it offers besides its default.
*/
func (c *TranslationController) States(ctx context.Context, in *requests.IndexTranslationStatesRequest,
) (*TranslationStatesOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	rows, err := c.Translations.States(ctx, membership(ctx), hc)
	if err != nil {
		return nil, err
	}
	kept := rows[:0]
	for _, r := range rows {
		if in.Locale != "" && r.Locale != in.Locale {
			continue
		}
		if in.State != "" && !inState(r, in.State) {
			continue
		}
		kept = append(kept, r)
	}
	items := resources.NewTranslationStates(kept)
	page := pagination.Page[resources.TranslationState]{Items: items, Limit: len(items)}
	return &TranslationStatesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
inState reports whether an article in a language is in the state asked
for; outdated is any translation behind its source.
*/
func inState(r helpcenters.TranslationState, state string) bool {
	if state == "outdated" {
		return r.Outdated
	}
	return r.State == state
}

/*
IndexCollection lists a collection's translations.
*/
func (c *TranslationController) IndexCollection(ctx context.Context, in *requests.CollectionRequest,
) (*CollectionTranslationsOutput, error) {
	id, err := c.decode(models.PrefixCollection, in.Collection)
	if err != nil {
		return nil, err
	}
	rows, err := c.Translations.ListCollection(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}
	items := resources.NewCollectionTranslations(rows)
	page := pagination.Page[resources.CollectionTranslation]{Items: items, Limit: helpcenters.MaxLocales}
	return &CollectionTranslationsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
IndexHelpCenterCollections lists every collection translation of a help
center.
*/
func (c *TranslationController) IndexHelpCenterCollections(ctx context.Context, in *requests.HelpCenterRequest,
) (*CollectionTranslationsOutput, error) {
	hc, err := c.decode(models.PrefixHelpCenter, in.HelpCenter)
	if err != nil {
		return nil, err
	}
	rows, err := c.Translations.ListForHelpCenter(ctx, membership(ctx), hc)
	if err != nil {
		return nil, err
	}
	items := resources.NewCollectionTranslations(rows)
	page := pagination.Page[resources.CollectionTranslation]{Items: items, Limit: len(items)}
	return &CollectionTranslationsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
UpdateCollection writes a collection's translation into a language.
*/
func (c *TranslationController) UpdateCollection(ctx context.Context, in *requests.UpdateCollectionTranslationRequest,
) (*CollectionTranslationOutput, error) {
	id, err := c.decode(models.PrefixCollection, in.Collection)
	if err != nil {
		return nil, err
	}
	t, err := c.Translations.SaveCollection(ctx, helpcenters.CollectionTranslationInput{
		Membership: membership(ctx), CollectionID: id, Locale: in.Locale, Name: in.Body.Name, Slug: in.Body.Slug,
		Description: in.Body.Description,
	})
	if err != nil {
		return nil, err
	}
	return &CollectionTranslationOutput{Body: fhttp.NewResource(resources.NewCollectionTranslation(t))}, nil
}

/*
DestroyCollection removes a collection's translation into a language.
*/
func (c *TranslationController) DestroyCollection(ctx context.Context, in *requests.CollectionTranslationRequest,
) (*struct{}, error) {
	id, err := c.decode(models.PrefixCollection, in.Collection)
	if err != nil {
		return nil, err
	}
	return &struct{}{}, c.Translations.DeleteCollection(ctx, membership(ctx), id, in.Locale)
}

/*
Languages lists the languages a help center may offer.
*/
func (c *TranslationController) Languages(_ context.Context, _ *requests.NoInput) (*LanguagesOutput, error) {
	items := resources.NewLanguages(helpcenters.Languages())
	page := pagination.Page[resources.Language]{Items: items, Limit: len(items)}
	return &LanguagesOutput{Body: fhttp.NewCollection(page)}, nil
}

func translationOutput(t helpcenters.Translation, err error) (*ArticleTranslationOutput, error) {
	if err != nil {
		return nil, err
	}
	return &ArticleTranslationOutput{Body: fhttp.NewResource(resources.NewArticleTranslation(t))}, nil
}

func (c *TranslationController) decode(prefix models.Prefix, public string) (ids.UUID, error) {
	id, err := c.IDs.Decode(string(prefix), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}
	return id, nil
}
