/*
|--------------------------------------------------------------------------
| GitHub Choices
|--------------------------------------------------------------------------
|
| The repository is picked from those the connected account can reach,
| most recently updated first, and the branch from the repository's
| branches with its default one preselected. The repository's folders
| and Markdown files are shown as a tree to tick.
|
*/

package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
maxPages bounds how many pages of repositories or branches are read.
*/
const maxPages = 10

/*
Options lists the repositories the account reaches or the branches of
the chosen repository.
*/
func (d *Driver) Options(ctx context.Context, cfg sources.Config, field string) ([]sources.Option, error) {
	switch field {
	case "repository":
		s, err := account(cfg)
		if err != nil {
			return nil, err
		}

		return d.repositories(ctx, s)
	case "branch":
		s, err := read(cfg)
		if err != nil {
			return nil, err
		}

		return d.branches(ctx, s)
	default:
		return nil, &sources.ConfigError{Field: field, Message: "This setting has no values to choose from."}
	}
}

/*
repositories lists every repository the account can read.
*/
func (d *Driver) repositories(ctx context.Context, s settings) ([]sources.Option, error) {
	out := []sources.Option{}

	for page := 1; page <= maxPages; page++ {
		var repos []struct {
			FullName    string `json:"full_name"`
			Private     bool   `json:"private"`
			Description string `json:"description"`
		}

		path := fmt.Sprintf("/user/repos?per_page=100&sort=updated&page=%d", page)
		if err := d.get(ctx, s, path, &repos); err != nil {
			return nil, err
		}

		for _, r := range repos {
			option := sources.Option{Value: r.FullName, Label: r.FullName, Detail: "Public"}
			if r.Private {
				option.Detail = "Private"
			}

			out = append(out, option)
		}

		if len(repos) < 100 {
			break
		}
	}

	return out, nil
}

/*
branches lists the repository's branches, the default one first and
preselected.
*/
func (d *Driver) branches(ctx context.Context, s settings) ([]sources.Option, error) {
	main, err := d.branch(ctx, settings{Repository: s.Repository, Token: s.Token})
	if err != nil {
		return nil, err
	}

	out := []sources.Option{{Value: main, Label: main, Detail: "Default branch", Default: true}}

	for page := 1; page <= maxPages; page++ {
		var branches []struct {
			Name string `json:"name"`
		}

		path := fmt.Sprintf("/repos/%s/branches?per_page=100&page=%d", s.Repository, page)
		if err := d.get(ctx, s, path, &branches); err != nil {
			return nil, err
		}

		for _, b := range branches {
			if b.Name != main {
				out = append(out, sources.Option{Value: b.Name, Label: b.Name})
			}
		}

		if len(branches) < 100 {
			break
		}
	}

	return out, nil
}

/*
Tree returns the branch's folders and Markdown files.
*/
func (d *Driver) Tree(ctx context.Context, cfg sources.Config) ([]sources.Node, error) {
	s, err := read(cfg)
	if err != nil {
		return nil, err
	}

	_, files, err := d.files(ctx, s)
	if err != nil {
		return nil, err
	}

	root := &folder{}
	for _, f := range files {
		root.add(strings.Split(f.Path, "/"), "")
	}

	return root.nodes(), nil
}

/*
folder gathers the folders and files under one folder while the tree is
built.
*/
type folder struct {
	path    string
	order   []string
	folders map[string]*folder
	files   []sources.Node
}

func (f *folder) add(parts []string, prefix string) {
	if len(parts) == 1 {
		f.files = append(f.files, sources.Node{
			ID: prefix + parts[0], Title: parts[0], Kind: "file", Children: []sources.Node{},
		})

		return
	}

	if f.folders == nil {
		f.folders = map[string]*folder{}
	}

	child, ok := f.folders[parts[0]]
	if !ok {
		child = &folder{path: prefix + parts[0]}
		f.folders[parts[0]] = child
		f.order = append(f.order, parts[0])
	}

	child.add(parts[1:], prefix+parts[0]+"/")
}

/*
nodes are the folder's folders, then its files.
*/
func (f *folder) nodes() []sources.Node {
	out := []sources.Node{}
	for _, name := range f.order {
		child := f.folders[name]
		out = append(out, sources.Node{
			ID: folderKey(child.path), Title: name, Kind: "folder", Children: child.nodes(),
		})
	}

	return append(out, f.files...)
}

func folderKey(path string) string {
	return "dir:" + path
}

/*
lineage is a file and every folder above it.
*/
func lineage(path string) []string {
	out := []string{path}

	for i := strings.LastIndex(path, "/"); i > 0; i = strings.LastIndex(path[:i], "/") {
		out = append(out, folderKey(path[:i]))
	}

	return out
}
