/*
|--------------------------------------------------------------------------
| Intercom Collections
|--------------------------------------------------------------------------
|
| An article sits in a collection, which may sit in another. The chain is
| read up from the article, at most three levels, in the article's own
| language when the collection has a translation, so each language's
| import builds collections named in that language.
|
*/

package intercom

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
maxLevels is how many collections above an article are read.
*/
const maxLevels = 3

/*
collection is an Intercom collection as the API returns it.
*/
type collection struct {
	ID                string                     `json:"id"`
	Name              string                     `json:"name"`
	Description       string                     `json:"description"`
	URL               string                     `json:"url"`
	ParentID          json.Number                `json:"parent_id"`
	TranslatedContent map[string]json.RawMessage `json:"translated_content"`
}

/*
collections reads the collections above the article, outermost first.
*/
func (d *Driver) collections(ctx context.Context, s settings, a article, locale string,
) ([]sources.Collection, error) {
	if a.ParentType != "collection" || a.ParentID.String() == "" {
		return nil, nil
	}

	var out []sources.Collection

	parent := a.ParentID.String()

	for range maxLevels {
		if parent == "" {
			break
		}

		var c collection

		err := d.get(ctx, s, "/help_center/collections/"+url.PathEscape(parent), &c)
		if errors.Is(err, sources.ErrNotFound) {
			break
		}

		if err != nil {
			return nil, err
		}

		out = append([]sources.Collection{c.in(locale)}, out...)
		parent = c.ParentID.String()
	}

	return out, nil
}

/*
in is the collection in the language, its own when it has no
translation.
*/
func (c collection) in(locale string) sources.Collection {
	out := sources.Collection{ExternalID: c.ID, Name: c.Name, Description: c.Description, URL: c.URL}

	raw, ok := c.TranslatedContent[locale]
	if !ok {
		return out
	}

	var t struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if json.Unmarshal(raw, &t) == nil && t.Name != "" {
		out.Name, out.Description = t.Name, t.Description
	}

	return out
}
