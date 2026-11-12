/*
|--------------------------------------------------------------------------
| Help Center Discovery
|--------------------------------------------------------------------------
|
| What crawlers and language models read of a help center: a sitemap of
| its published public articles, collections, changelog entries and the
| open feature request board, each with when it last changed; a robots
| file pointing at it, closing the site when the team turned indexing
| off; and an llms.txt index in the llmstxt.org format linking every
| article's Markdown, with llms-full.txt carrying the Markdown itself.
| Every address is the help center's canonical one, its custom domain
| once verified. These belong to each help center's own host or path,
| apart from the app's own /llms.txt.
|
*/

package web

import (
	"encoding/xml"
	"strings"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/models"
)

/*
sitemapChangelogEntries caps the changelog entries a sitemap lists.
*/
const sitemapChangelogEntries = 500

/*
sitemapURL is one address of a sitemap.
*/
type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

/*
sitemapSet is a sitemap's addresses.
*/
type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	NS      string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

/*
discoverable finds the real help center the request names as anyone
sees it, answering not found for the sample or an unknown one.
*/
func (c *HelpCenterController) discoverable(w fhttp.ResponseWriter, r *fhttp.Request,
) (helpcenters.Snapshot, string, bool) {
	if c.Public == nil {
		w.WriteHeader(fhttp.StatusNotFound)
		return helpcenters.Snapshot{}, "", false
	}
	snap, err := c.Public.Load(r.Context(), r.PathValue("center"))
	if err != nil {
		w.WriteHeader(fhttp.StatusNotFound)
		return snap, "", false
	}
	return snap, c.Addresses.Root(snap.HelpCenter), true
}

/*
Sitemap lists the help center's public pages for crawlers.
*/
func (c *HelpCenterController) Sitemap(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, root, ok := c.discoverable(w, r)
	if !ok {
		return
	}
	if !snap.HelpCenter.IndexingEnabled {
		w.WriteHeader(fhttp.StatusNotFound)
		return
	}
	set := sitemapSet{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	set.URLs = append(set.URLs, sitemapURL{Loc: root + "/", LastMod: day(newest(snap))})
	for _, col := range snap.Collections {
		set.URLs = append(set.URLs, sitemapURL{Loc: root + "/collections/" + col.Slug, LastMod: day(col.UpdatedAt)})
	}
	for _, a := range snap.Articles {
		set.URLs = append(set.URLs, sitemapURL{Loc: root + "/articles/" + a.Slug, LastMod: day(a.VersionCreatedAt)})
	}
	set.URLs = append(set.URLs, c.changelogURLs(r, snap.HelpCenter, root)...)
	if helpcenters.SettingsOf(snap.HelpCenter).FeedbackBoard {
		set.URLs = append(set.URLs, sitemapURL{Loc: root + "/feedback"})
	}
	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		c.Errors.Render(w, r, err)
		return
	}
	writeText(w, "application/xml; charset=utf-8", xml.Header+string(body)+"\n")
}

/*
changelogURLs lists the changelog and its entries when it has any.
*/
func (c *HelpCenterController) changelogURLs(r *fhttp.Request, h models.HelpCenter, root string) []sitemapURL {
	if c.Changelog == nil {
		return nil
	}
	entries, err := c.Changelog.Latest(r.Context(), h, sitemapChangelogEntries)
	if err != nil || len(entries) == 0 {
		return nil
	}
	var out []sitemapURL
	for i, e := range entries {
		at := e.UpdatedAt
		if e.PublishedAt != nil {
			at = *e.PublishedAt
		}
		if i == 0 {
			out = append(out, sitemapURL{Loc: root + "/changelog", LastMod: day(at)})
		}
		out = append(out, sitemapURL{Loc: root + "/changelog/" + e.Slug, LastMod: day(at)})
	}
	return out
}

/*
Robots tells crawlers where the sitemap is, or to stay away when the team
turned indexing off.
*/
func (c *HelpCenterController) Robots(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, root, ok := c.discoverable(w, r)
	if !ok {
		return
	}
	if !snap.HelpCenter.IndexingEnabled {
		writeText(w, "text/plain; charset=utf-8", "User-agent: *\nDisallow: /\n")
		return
	}
	writeText(w, "text/plain; charset=utf-8", "User-agent: *\nAllow: /\nDisallow: /search\n"+
		"Disallow: /sso\nDisallow: /sign-in\nDisallow: /conversations\n\nSitemap: "+root+"/sitemap.xml\n")
}

/*
LLMs answers the llms.txt index: the help center's name and summary,
then its articles by collection, each linking its Markdown.
*/
func (c *HelpCenterController) LLMs(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, root, ok := c.discoverable(w, r)
	if !ok {
		return
	}
	var b strings.Builder
	b.WriteString(llmsHeading(snap))
	listed := map[string]bool{}
	for _, col := range snap.Collections {
		var lines []string
		for _, a := range snap.Articles {
			if a.CollectionID != nil && *a.CollectionID == col.ID {
				lines, listed[a.Slug] = append(lines, llmsLink(root, a)), true
			}
		}
		if len(lines) > 0 {
			b.WriteString("\n## " + oneLine(col.Name) + "\n\n" + strings.Join(lines, "\n") + "\n")
		}
	}
	var rest []string
	for _, a := range snap.Articles {
		if !listed[a.Slug] {
			rest = append(rest, llmsLink(root, a))
		}
	}
	if len(rest) > 0 {
		b.WriteString("\n## Articles\n\n" + strings.Join(rest, "\n") + "\n")
	}
	b.WriteString("\n## Optional\n\n- [Full text](" + root + "/llms-full.txt): every article's Markdown in one file\n")
	writeText(w, "text/plain; charset=utf-8", b.String())
}

/*
LLMsFull answers every article's Markdown in one file, after the same
heading as llms.txt.
*/
func (c *HelpCenterController) LLMsFull(w fhttp.ResponseWriter, r *fhttp.Request) {
	snap, root, ok := c.discoverable(w, r)
	if !ok {
		return
	}
	var b strings.Builder
	b.WriteString(llmsHeading(snap))
	for _, a := range snap.Articles {
		b.WriteString("\n---\n\n# " + oneLine(a.Title) + "\n\nSource: " + root + "/articles/" + a.Slug + "\n\n")
		if a.Description != nil && *a.Description != "" {
			b.WriteString("> " + oneLine(*a.Description) + "\n\n")
		}
		b.WriteString(strings.TrimSpace(a.BodyMarkdown) + "\n")
	}
	writeText(w, "text/plain; charset=utf-8", b.String())
}

/*
llmsHeading is the title and summary an llms.txt file starts with.
*/
func llmsHeading(snap helpcenters.Snapshot) string {
	h := snap.HelpCenter
	summary := helpcenters.SettingsOf(h).HomeSubtitle
	if summary == "" {
		summary = "Help articles and guides from " + h.Name + "."
	}
	return "# " + oneLine(h.Name) + "\n\n> " + oneLine(summary) + "\n"
}

/*
llmsLink is an article's line in llms.txt, linking its Markdown.
*/
func llmsLink(root string, a models.ListPublishedArticlesRow) string {
	line := "- [" + oneLine(a.Title) + "](" + root + "/articles/" + a.Slug + ".md)"
	if a.Description != nil && *a.Description != "" {
		line += ": " + oneLine(*a.Description)
	}
	return line
}

/*
oneLine keeps a text on one line, so it never starts a heading or link
of its own.
*/
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

/*
newest is when the help center's newest article version was published.
*/
func newest(snap helpcenters.Snapshot) time.Time {
	out := snap.HelpCenter.UpdatedAt
	for _, a := range snap.Articles {
		if a.VersionCreatedAt.After(out) {
			out = a.VersionCreatedAt
		}
	}
	return out
}

/*
day writes a moment as a sitemap date.
*/
func day(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.DateOnly)
}

/*
writeText sends a text answer crawlers may keep for a while.
*/
func writeText(w fhttp.ResponseWriter, kind, body string) {
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(fhttp.StatusOK)
	_, _ = w.Write([]byte(body))
}

/*
canonical is the page's address under the help center's canonical root,
its custom domain once verified, whichever address it was read at.
*/
func (c *HelpCenterController) canonical(r *fhttp.Request, h models.HelpCenter) string {
	rest := strings.TrimPrefix(r.URL.Path, "/hc/"+r.PathValue("center"))
	return c.Addresses.Root(h) + rest
}
