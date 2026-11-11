/*
|--------------------------------------------------------------------------
| Notion Page Tree
|--------------------------------------------------------------------------
|
| The pages shared with the connection, arranged as Notion nests them,
| for the member to tick the ones to read. A page whose parent was not
| shared sits at the top. Ticking a page reads the pages under it too.
|
*/

package notion

import (
	"context"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
Tree returns the shared pages as Notion nests them.
*/
func (d *Driver) Tree(ctx context.Context, cfg sources.Config) ([]sources.Node, error) {
	s, err := read(cfg)
	if err != nil {
		return nil, err
	}

	all, err := d.pages(ctx, s)
	if err != nil {
		return nil, err
	}

	children := map[string][]page{}
	known := map[string]bool{}

	for _, p := range all {
		known[PageID(p.ID)] = true
	}

	for _, p := range all {
		parent := ""
		if p.Parent.Type == "page_id" && known[PageID(p.Parent.PageID)] {
			parent = PageID(p.Parent.PageID)
		}

		children[parent] = append(children[parent], p)
	}

	return nodes(children, "", 0), nil
}

/*
nodes builds the tree under a page, ordered by title.
*/
func nodes(children map[string][]page, parent string, depth int) []sources.Node {
	pages := children[parent]
	slices.SortFunc(pages, func(a, b page) int { return strings.Compare(a.title(), b.title()) })

	out := make([]sources.Node, 0, len(pages))
	for _, p := range pages {
		n := sources.Node{ID: PageID(p.ID), Title: p.title(), Kind: "page", Children: []sources.Node{}}
		if depth < 10 {
			n.Children = nodes(children, PageID(p.ID), depth+1)
		}

		out = append(out, n)
	}

	return out
}

/*
parentsOf maps each shared page to the shared page it sits in.
*/
func parentsOf(all []page) map[string]string {
	out := map[string]string{}
	for _, p := range all {
		if p.Parent.Type == "page_id" {
			out[PageID(p.ID)] = PageID(p.Parent.PageID)
		}
	}

	return out
}

/*
lineage is a page and the pages above it, nearest first.
*/
func lineage(parents map[string]string, id string) []string {
	out := []string{id}
	for range 20 {
		up, ok := parents[id]
		if !ok {
			break
		}

		out = append(out, up)
		id = up
	}

	return out
}
