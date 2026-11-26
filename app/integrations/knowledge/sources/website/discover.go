/*
|--------------------------------------------------------------------------
| Website Discovery
|--------------------------------------------------------------------------
|
| A listing is the site's sitemap: the one the settings name, else the
| site's /sitemap.xml, following sitemap indexes and gzipped files, up
| to fifty sitemaps. Only addresses on the site's own host that
| robots.txt allows and the member chose are kept, each once, up to
| the page limit, the pages nearest the top of the site first, so a
| limit keeps the home page and its sections rather than whatever the
| sitemap happens to list first. A site without a sitemap is listed as
| its start page alone, and the crawl finds the rest through links.
|
*/

package website

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"io"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/integrations/knowledge/sources"
)

/*
maxSitemaps is how many sitemap files one listing reads.
*/
const maxSitemaps = 50

/*
sitemapFile is a sitemap or a sitemap index; only one of the lists is
filled.
*/
type sitemapFile struct {
	URLs     []sitemapEntry `xml:"url"`
	Sitemaps []sitemapEntry `xml:"sitemap"`
}

/*
sitemapEntry is one address of a sitemap and when it last changed.
*/
type sitemapEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

/*
List returns every page the sitemap allows in one page. A site without
a sitemap is its start page and the pages that one links to. Pages a
member ticked one by one are listed whether or not the site names them.
*/
func (d *Builtin) List(ctx context.Context, s Site, _ string) (sources.Page, error) {
	start, _ := url.Parse(s.StartURL)

	entries, err := d.sitemapURLs(ctx, s, start)
	if err != nil {
		return sources.Page{}, err
	}

	if len(entries) == 0 {
		entries = d.linked(ctx, s)
	}

	for _, pattern := range s.Include {
		if !strings.ContainsAny(pattern, "*?[") {
			entries = append(entries, sitemapEntry{
				Loc: start.Scheme + "://" + start.Host + "/" + strings.TrimPrefix(pattern, "/"),
			})
		}
	}

	return sources.Page{Refs: d.refs(ctx, s, start, entries)}, nil
}

/*
linked is the start page and the pages it links to, what is known of a
site that has no sitemap.
*/
func (d *Builtin) linked(ctx context.Context, s Site) []sitemapEntry {
	out := []sitemapEntry{{Loc: s.StartURL}}

	doc, err := d.Fetch(ctx, s, sources.Ref{URL: s.StartURL})
	if err != nil {
		return out
	}

	for _, link := range doc.Links {
		out = append(out, sitemapEntry{Loc: link})
	}

	return out
}

/*
depth is how far below the top of its site an address sits: none for
the home page, one for /pricing, two for /docs/refunds.
*/
func depth(address string) int {
	u, err := url.Parse(strings.TrimSpace(address))
	if err != nil {
		return 0
	}

	path := strings.Trim(u.Path, "/")
	if path == "" {
		return 0
	}

	return strings.Count(path, "/") + 1
}

/*
refs keeps the entries on the site that robots.txt allows and the
member chose, each once, nearest the top of the site first, up to the
page limit. Pages a member's exclude pattern leaves out stay listed:
the engine sets them aside with that reason, where the member sees it.
*/
func (d *Builtin) refs(ctx context.Context, s Site, start *url.URL, entries []sitemapEntry) []sources.Ref {
	seen, limit := map[string]bool{}, s.MaxPages

	entries = slices.Clone(entries)
	slices.SortStableFunc(entries, func(a, b sitemapEntry) int { return depth(a.Loc) - depth(b.Loc) })

	var out []sources.Ref

	for _, e := range entries {
		u, err := url.Parse(strings.TrimSpace(e.Loc))
		if err != nil || !sameSite(start, u) || !sources.Allowed(s.Include, nil, u.EscapedPath()) {
			continue
		}

		address := Canonical(u)
		if seen[address] {
			continue
		}

		if ok, err := d.allowed(ctx, u); err != nil || !ok {
			continue
		}

		seen[address] = true
		out = append(out, sources.Ref{
			ExternalID: address, URL: address, Path: u.EscapedPath(), Version: e.LastMod, UpdatedAt: lastMod(e.LastMod),
		})

		if len(out) >= limit {
			break
		}
	}

	return out
}

/*
sitemapURLs reads the sitemap the settings name, else /sitemap.xml,
into its page addresses. A site without one has none.
*/
func (d *Builtin) sitemapURLs(ctx context.Context, s Site, start *url.URL) ([]sitemapEntry, error) {
	first := s.SitemapURL
	if first == "" {
		first = start.Scheme + "://" + start.Host + "/sitemap.xml"
	}

	queue, read := []string{first}, 0

	var out []sitemapEntry

	for len(queue) > 0 && read < maxSitemaps {
		next := queue[0]
		queue = queue[1:]
		read++

		file, err := d.sitemap(ctx, next)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if err != nil {
			continue
		}

		out = append(out, file.URLs...)
		for _, child := range file.Sitemaps {
			queue = append(queue, strings.TrimSpace(child.Loc))
		}
	}

	return out, nil
}

/*
sitemap reads one sitemap file, gzipped or not.
*/
func (d *Builtin) sitemap(ctx context.Context, address string) (sitemapFile, error) {
	var file sitemapFile

	resp, err := d.get(ctx, address)
	if err != nil {
		return file, err
	}

	if err := sources.FromResponse(resp, "read sitemap"); err != nil {
		return file, err
	}

	body := resp.Body
	if bytes.HasPrefix(body, []byte{0x1f, 0x8b}) {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return file, err
		}

		if body, err = io.ReadAll(io.LimitReader(zr, maxPageBytes*10)); err != nil {
			return file, err
		}
	}

	err = xml.Unmarshal(body, &file)

	return file, err
}

/*
lastMod reads a sitemap date, zero when it has none.
*/
func lastMod(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t
		}
	}

	return time.Time{}
}
