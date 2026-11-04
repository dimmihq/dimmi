/*
|--------------------------------------------------------------------------
| Sample Help Center
|--------------------------------------------------------------------------
|
| A help center and changelog written out as Markdown files, so the public
| pages can be seen and judged before articles and entries live in the
| database. It holds dimmi's own help center, as the product will publish
| it, and a second help center in another brand color to show how the
| pages follow a customer's brand. Everything is read and checked once at
| boot; a broken file stops the boot rather than a page.
|
*/

package helpsample

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/markdown"

	"github.com/dimmihq/dimmi/resources/views/helpcenter"
)

//go:embed content
var content embed.FS

/*
Library is the sample content: the help centers, collections and
articles, the changelog entries, the feature requests and the signed-in
visitor's conversations.
*/
type Library struct {
	sites         []site
	collections   []collection
	articles      []article
	entries       []entry
	statuses      []status
	conversations []conversation
}

/*
Load reads and checks the sample content.
*/
func Load() (*Library, error) {
	lib := &Library{
		sites:       sites(),
		collections: collections(),
	}

	articles, err := readArticles()
	if err != nil {
		return nil, err
	}

	entries, err := readEntries()
	if err != nil {
		return nil, err
	}

	lib.articles, lib.entries = articles, entries

	if err := readJSON("feedback.json", &lib.statuses); err != nil {
		return nil, err
	}

	if err := readJSON("conversations.json", &lib.conversations); err != nil {
		return nil, err
	}

	if err := lib.check(); err != nil {
		return nil, err
	}

	return lib, nil
}

/*
Assets returns the sample images, such as changelog covers.
*/
func Assets() fs.FS {
	sub, err := fs.Sub(content, "content/assets")
	if err != nil {
		return content
	}

	return sub
}

/*
Center returns the help center with the slug, with its paths under base.
*/
func (l *Library) Center(slug, base string) (helpcenter.Center, bool) {
	for _, s := range l.sites {
		if s.slug == slug {
			c := s.center
			c.Base = base
			c.Sections = []helpcenter.Section{
				{Key: "changelog", Name: "Changelog", Icon: "megaphone", Path: "/changelog"},
				{Key: "feedback", Name: "Feature requests", Icon: "lightbulb", Path: "/feedback"},
				{Key: "conversations", Name: "Conversations", Icon: "inbox", Path: "/conversations"},
			}

			return c, true
		}
	}

	return helpcenter.Center{}, false
}

/*
check makes sure every article sits in a known collection, every link
between files points somewhere and every entry has known authors.
*/
func (l *Library) check() error {
	for _, a := range l.articles {
		if _, ok := l.findCollection(a.collection); !ok {
			return fmt.Errorf("article %s: unknown collection %q", a.slug, a.collection)
		}

		if _, ok := person(a.author); !ok {
			return fmt.Errorf("article %s: unknown author %q", a.slug, a.author)
		}
	}

	for _, e := range l.entries {
		for _, slug := range e.related {
			if _, ok := l.findArticle(slug); !ok {
				return fmt.Errorf("entry %s: unknown article %q", e.slug, slug)
			}
		}
	}

	return nil
}

/*
render turns Markdown into HTML, with links to other pages moved under
the help center's base.
*/
func (l *Library) render(c helpcenter.Center, src string) (markdown.Result, error) {
	r := markdown.New(markdown.Options{
		GFM: true, Footnotes: true, Alerts: true, HeadingIDs: true, Highlight: true,
		RewriteLink: func(link markdown.Link) markdown.Link {
			if strings.HasPrefix(link.URL, "/") {
				link.URL = c.Base + link.URL
			}

			return link
		},
	})

	res, err := r.HTML([]byte(src))
	if err != nil {
		return markdown.Result{}, fmt.Errorf("render sample markdown: %w", err)
	}

	return res, nil
}

/*
readDir reads every Markdown file of a content folder, sorted by name,
as front matter and body.
*/
func readDir(dir string, each func(slug string, meta map[string]any, body string) error) error {
	names, err := fs.Glob(content, path.Join("content", dir, "*.md"))
	if err != nil {
		return fmt.Errorf("list sample %s: %w", dir, err)
	}

	slices.Sort(names)

	for _, name := range names {
		src, err := content.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read sample %s: %w", name, err)
		}

		meta, body, err := markdown.FrontMatter(src, 0)
		if err != nil {
			return fmt.Errorf("read sample %s: %w", name, err)
		}

		if err := each(strings.TrimSuffix(path.Base(name), ".md"), meta, string(body)); err != nil {
			return fmt.Errorf("read sample %s: %w", name, err)
		}
	}

	return nil
}
