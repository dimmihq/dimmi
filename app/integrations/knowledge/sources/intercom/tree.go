/*
|--------------------------------------------------------------------------
| Intercom Tree
|--------------------------------------------------------------------------
|
| The help center as Intercom arranges it: collections inside
| collections, articles inside them, and articles in no collection at
| the top. Ticking a collection reads every article under it, at any
| depth, including articles added to it later.
|
*/

package intercom

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
Tree returns the collections and articles as Intercom nests them.
*/
func (d *Driver) Tree(ctx context.Context, cfg sources.Config) ([]sources.Node, error) {
	s, err := read(cfg)
	if err != nil {
		return nil, err
	}

	collections, err := d.allCollections(ctx, s)
	if err != nil {
		return nil, err
	}

	articles, err := d.allArticles(ctx, s)
	if err != nil {
		return nil, err
	}

	under := map[string][]sources.Node{}
	for _, a := range articles {
		parent := a.ParentID.String()
		under[parent] = append(under[parent], sources.Node{
			ID: a.ID, Title: a.Title, Kind: "article", Children: []sources.Node{},
		})
	}

	return branch(collections, under, "", 0), nil
}

/*
branch builds the tree under a collection, collections first.
*/
func branch(collections []collection, articles map[string][]sources.Node, parent string, depth int,
) []sources.Node {
	out := []sources.Node{}

	for _, c := range collections {
		if c.ParentID.String() != parent || depth > 5 {
			continue
		}

		out = append(out, sources.Node{
			ID: c.ID, Title: c.Name, Kind: "collection", Children: branch(collections, articles, c.ID, depth+1),
		})
	}

	return append(out, articles[parent]...)
}

/*
allCollections reads every collection.
*/
func (d *Driver) allCollections(ctx context.Context, s settings) ([]collection, error) {
	var out []collection

	for page := 1; page <= 50; page++ {
		var list struct {
			Data  []collection `json:"data"`
			Pages struct {
				TotalPages int `json:"total_pages"`
			} `json:"pages"`
		}

		if err := d.get(ctx, s, fmt.Sprintf("/help_center/collections?page=%d&per_page=150", page), &list); err != nil {
			return nil, err
		}

		out = append(out, list.Data...)
		if page >= list.Pages.TotalPages {
			break
		}
	}

	return out, nil
}

/*
allArticles reads every article.
*/
func (d *Driver) allArticles(ctx context.Context, s settings) ([]article, error) {
	var out []article

	for page := 1; page <= 200; page++ {
		var list listing
		if err := d.get(ctx, s, "/articles?page="+strconv.Itoa(page)+"&per_page=150", &list); err != nil {
			return nil, err
		}

		out = append(out, list.Data...)
		if page >= list.Pages.TotalPages {
			break
		}
	}

	return out, nil
}

/*
lineageOf is an article and every collection above it.
*/
func lineageOf(a article) []string {
	out := []string{a.ID}
	for _, id := range a.ParentIDs {
		out = append(out, id.String())
	}

	if p := a.ParentID.String(); p != "" {
		out = append(out, p)
	}

	return out
}
