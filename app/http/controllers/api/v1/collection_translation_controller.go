/*
|--------------------------------------------------------------------------
| Collection Translation Controller
|--------------------------------------------------------------------------
|
| A collection's name, slug and description in the other languages its
| help center offers, and the languages a help center may offer at all.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

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
