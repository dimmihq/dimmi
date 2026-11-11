/*
|--------------------------------------------------------------------------
| Zendesk Sections
|--------------------------------------------------------------------------
|
| A Zendesk article sits in a section, which may sit in a parent
| section, all inside a category. They are read in the article's own
| language, so a translation's collections carry translated names, and
| become the article's collections, category first.
|
*/

package zendesk

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
maxSections is how many nested sections above an article are read.
*/
const maxSections = 3

/*
section is a Zendesk section or category.
*/
type section struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	HTMLURL         string `json:"html_url"`
	CategoryID      int64  `json:"category_id"`
	ParentSectionID *int64 `json:"parent_section_id"`
}

/*
collections reads the sections and category above the article,
outermost first.
*/
func (d *Driver) collections(ctx context.Context, s settings, a article) ([]sources.Collection, error) {
	if a.SectionID == 0 {
		return nil, nil
	}

	var out []sources.Collection

	next, category := &a.SectionID, int64(0)

	for i := 0; i < maxSections && next != nil; i++ {
		var one struct {
			Section section `json:"section"`
		}

		err := d.get(ctx, s, fmt.Sprintf("%s/%s/sections/%d.json", s.base(), url.PathEscape(a.Locale), *next), &one)
		if errors.Is(err, sources.ErrNotFound) {
			break
		}

		if err != nil {
			return nil, err
		}

		out = append([]sources.Collection{collectionOf(one.Section)}, out...)
		next, category = one.Section.ParentSectionID, one.Section.CategoryID
	}

	if category == 0 {
		return out, nil
	}

	var one struct {
		Category section `json:"category"`
	}

	err := d.get(ctx, s, fmt.Sprintf("%s/%s/categories/%d.json", s.base(), url.PathEscape(a.Locale), category), &one)
	if err != nil && !errors.Is(err, sources.ErrNotFound) {
		return nil, err
	}

	if err == nil {
		out = append([]sources.Collection{collectionOf(one.Category)}, out...)
	}

	return out, nil
}

func collectionOf(s section) sources.Collection {
	return sources.Collection{
		ExternalID: fmt.Sprint(s.ID), Name: s.Name, Description: s.Description, URL: s.HTMLURL,
	}
}
