/*
|--------------------------------------------------------------------------
| Website Tree
|--------------------------------------------------------------------------
|
| A website's pages are shown as a tree of their addresses, folder by
| folder, so a member ticks the parts of the site to read instead of
| typing patterns. The pages come from the site's sitemaps, or from the
| links on its start page when it has none. Each item's id is the path
| pattern it stands for, a folder "/docs/**" and a page its own path,
| and the ticks are kept as the source's include patterns, so the crawl
| and every link it follows keep to them.
|
*/

package website

import (
	"context"
	"net/url"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
treePages bounds how many pages the tree shows.
*/
const treePages = 2000

/*
Tree returns the site's pages grouped by folder.
*/
func (d *Driver) Tree(ctx context.Context, cfg sources.Config) ([]sources.Node, error) {
	s, err := read(cfg)
	if err != nil {
		return nil, err
	}

	s.Include, s.Exclude, s.MaxPages = nil, nil, treePages

	paths, err := d.pagePaths(ctx, s)
	if err != nil {
		return nil, err
	}

	root := &folder{}
	for _, p := range paths {
		root.add(strings.Split(strings.Trim(p, "/"), "/"), "")
	}

	return root.nodes(), nil
}

/*
pagePaths lists the paths of the site's pages: from its sitemaps, or
the start page and the pages it links to.
*/
func (d *Driver) pagePaths(ctx context.Context, s Site) ([]string, error) {
	page, err := d.builtin.List(ctx, s, "")
	if err != nil {
		return nil, err
	}

	addresses := make([]string, 0, len(page.Refs))
	for _, r := range page.Refs {
		addresses = append(addresses, r.URL)
	}

	if len(page.Refs) == 1 {
		doc, err := d.builtin.Fetch(ctx, s, page.Refs[0])
		if err == nil {
			addresses = append(addresses, doc.Links...)
		}
	}

	start, _ := url.Parse(s.StartURL)

	var out []string
	for _, a := range addresses {
		u, err := url.Parse(a)
		if err != nil || !sameSite(start, u) {
			continue
		}

		if p := strings.TrimSuffix(u.Path, "/"); p != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}

	return out, nil
}

/*
folder gathers the folders and pages under one folder while the tree
is built.
*/
type folder struct {
	path    string
	order   []string
	folders map[string]*folder
	pages   []sources.Node
}

func (f *folder) add(parts []string, prefix string) {
	if len(parts) == 1 {
		f.pages = append(f.pages, sources.Node{
			ID: prefix + "/" + parts[0], Title: parts[0], Kind: "page", Children: []sources.Node{},
		})

		return
	}

	if f.folders == nil {
		f.folders = map[string]*folder{}
	}

	child, ok := f.folders[parts[0]]
	if !ok {
		child = &folder{path: prefix + "/" + parts[0]}
		f.folders[parts[0]] = child
		f.order = append(f.order, parts[0])
	}

	child.add(parts[1:], prefix+"/"+parts[0])
}

/*
nodes are the folder's folders, then its pages. A folder that is also a
page, such as /docs beside /docs/refunds, shows once, as the folder.
*/
func (f *folder) nodes() []sources.Node {
	out := []sources.Node{}
	for _, name := range f.order {
		child := f.folders[name]
		out = append(out, sources.Node{
			ID: child.path + "/**", Title: name, Kind: "folder", Children: child.nodes(),
		})
	}

	for _, p := range f.pages {
		if _, isFolder := f.folders[p.Title]; !isFolder {
			out = append(out, p)
		}
	}

	return out
}
