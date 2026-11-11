/*
|--------------------------------------------------------------------------
| Website Pages
|--------------------------------------------------------------------------
|
| A page is read only where robots.txt allows. Its canonical link wins:
| a page whose canonical address is another one is a duplicate, and the
| canonical address is crawled instead. The title comes from og:title,
| else the main content's title, else the <title> without the site's
| suffix. A page whose main content is nearly empty while it ships an
| app shell or a "turn on JavaScript" notice renders in the browser,
| and is set aside with that reason. Links on the site are handed back
| for the crawl unless the page asks robots not to follow them.
|
*/

package website

import (
	"context"
	"net/url"
	"strings"

	"github.com/dimmihq/dimmi/framework/dom"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
minText is the least readable text a server-rendered page has.
*/
const minText = 200

/*
Fetch reads one page.
*/
func (d *Builtin) Fetch(ctx context.Context, s Site, ref sources.Ref) (sources.Document, error) {
	u, err := url.Parse(ref.URL)
	if err != nil {
		return sources.Document{}, sources.ErrNotFound
	}

	if ok, err := d.allowed(ctx, u); err != nil || !ok {
		return sources.Document{}, sources.Excluded("robots")
	}

	resp, err := d.get(ctx, ref.URL)
	if err != nil {
		return sources.Document{}, err
	}

	if err := sources.FromResponse(resp, "read page"); err != nil {
		return sources.Document{}, err
	}

	kind, _, _ := strings.Cut(strings.ToLower(resp.Header.Get("Content-Type")), ";")
	kind = strings.TrimSpace(kind)
	switch kind {
	case "text/markdown", "text/plain", "text/x-markdown":
		doc := sources.MarkdownFile(u.Path, resp.Body)
		doc.URL, doc.Visibility, doc.Path = ref.URL, sources.VisibilityPublic, u.EscapedPath()

		return doc, nil
	case "text/html", "application/xhtml+xml", "":
		return d.page(u, string(resp.Body), s)
	}

	return sources.Document{}, sources.Excluded("unsupported_type")
}

/*
page turns an HTML page into a document.
*/
func (d *Builtin) page(u *url.URL, html string, s Site) (sources.Document, error) {
	doc, err := dom.Parse(strings.NewReader(html))
	if err != nil {
		return sources.Document{}, sources.Excluded("unsupported_type")
	}

	address := Canonical(u)
	links := []string(nil)

	if s.Follows() && !nofollow(doc) {
		links = pageLinks(doc, u)
	}

	if canonical := canonicalOf(doc, u); canonical != "" && canonical != address {
		return sources.Document{Links: []string{canonical}}, sources.Excluded("duplicate")
	}

	readable, err := dom.Readable(html, address)
	if err != nil || len(strings.TrimSpace(readable.Text)) < minText {
		if appShell(doc) {
			return sources.Document{Links: links}, sources.Excluded("js_rendered")
		}

		if err != nil {
			return sources.Document{Links: links}, sources.Excluded("empty")
		}
	}

	md, err := dom.ToMarkdown(readable.HTML)
	if err != nil || strings.TrimSpace(md) == "" {
		return sources.Document{Links: links}, sources.Excluded("empty")
	}

	return sources.Document{
		Title: title(doc, readable.Title, u), Markdown: sources.Tidy(md), URL: address, Path: u.EscapedPath(),
		Locale: readable.Language, Visibility: sources.VisibilityPublic, Links: links, Description: readable.Excerpt,
	}, nil
}

/*
canonicalOf returns the page's canonical address when it is on the
same site, else empty.
*/
func canonicalOf(doc *dom.Document, u *url.URL) string {
	href, ok := doc.Find(`link[rel="canonical"]`).Attr("href")
	if !ok || strings.TrimSpace(href) == "" {
		return ""
	}

	target, err := u.Parse(strings.TrimSpace(href))
	if err != nil || !sameSite(u, target) {
		return ""
	}

	return Canonical(target)
}

/*
nofollow reports whether the page asks robots not to follow its links.
*/
func nofollow(doc *dom.Document) bool {
	content, _ := doc.Find(`meta[name="robots"]`).Attr("content")

	return strings.Contains(strings.ToLower(content), "nofollow")
}

/*
pageLinks lists the page's links on the same site, canonical.
*/
func pageLinks(doc *dom.Document, u *url.URL) []string {
	seen := map[string]bool{}

	var out []string

	doc.Find("a[href]").Each(func(s dom.Selection) {
		href, _ := s.Attr("href")

		target, err := u.Parse(strings.TrimSpace(href))
		if err != nil || !sameSite(u, target) {
			return
		}

		address := Canonical(target)
		if !seen[address] {
			seen[address] = true
			out = append(out, address)
		}
	})

	return out
}

/*
appShell reports whether a page looks rendered by JavaScript: a notice
asking for JavaScript, or an empty app root.
*/
func appShell(doc *dom.Document) bool {
	if strings.Contains(strings.ToLower(doc.Find("noscript").Text()), "javascript") {
		return true
	}

	for _, root := range []string{"#root", "#__next", "#app", "#__nuxt"} {
		el := doc.Find(root)
		if el.Len() > 0 && strings.TrimSpace(el.Text()) == "" {
			return true
		}
	}

	return false
}

/*
title picks the page's title: og:title, else the main heading, else the
readable or <title> title without a " | Site" or " - Site" suffix, else
the path.
*/
func title(doc *dom.Document, readable string, u *url.URL) string {
	if og, ok := doc.Find(`meta[property="og:title"]`).Attr("content"); ok && strings.TrimSpace(og) != "" {
		return withoutSite(og)
	}

	if h1 := strings.TrimSpace(doc.Find("h1").Text()); h1 != "" && len(h1) < 200 {
		return h1
	}

	for _, t := range []string{readable, doc.Find("title").Text()} {
		if t = withoutSite(t); t != "" {
			return t
		}
	}

	return sources.Humanize(u.Path)
}

/*
withoutSite drops a trailing site name from a title.
*/
func withoutSite(t string) string {
	t = strings.TrimSpace(t)
	for _, sep := range []string{" | ", " - ", " — ", " · "} {
		if i := strings.LastIndex(t, sep); i > 0 {
			t = t[:i]
		}
	}

	return strings.TrimSpace(t)
}
