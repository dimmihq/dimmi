/*
|--------------------------------------------------------------------------
| Article Rendering
|--------------------------------------------------------------------------
|
| Articles are Markdown: CommonMark with tables, task lists, footnotes
| and alerts. Each published version is rendered to sanitized HTML once,
| with its table of contents, so serving a page renders nothing. The
| renderer version is stored with each version, so a change to the
| rendering can re-render old versions instead of serving stale HTML.
| The content hash covers everything a reader could see change, and
| tells whether the working copy differs from what is live.
|
*/

package helpcenters

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/markdown"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/models"
)

/*
RendererVersion is the version of the rendering stored with each
article version.
*/
const RendererVersion = 2

/*
rendered is an article body as a version keeps it.
*/
type rendered struct {
	html string
	toc  json.RawMessage
}

/*
renderer turns article Markdown into the HTML readers see.
*/
func renderer() *markdown.Renderer {
	return markdown.New(markdown.Options{
		GFM: true, Footnotes: true, Alerts: true, HeadingIDs: true, ImageLayout: true, Highlight: true,
	})
}

/*
refreshed renders again the published articles an older renderer kept,
such as before code was highlighted, so readers never see stale HTML.
An article that fails to render keeps what it had.
*/
func refreshed(rows []models.ListPublishedArticlesRow) []models.ListPublishedArticlesRow {
	var r *markdown.Renderer
	for i, row := range rows {
		if row.RendererVersion >= RendererVersion {
			continue
		}
		if r == nil {
			r = renderer()
		}
		if out, err := render(r, row.BodyMarkdown); err == nil {
			rows[i].BodyHtml, rows[i].Toc = out.html, out.toc
		}
	}
	return rows
}

/*
render renders a body and its table of contents.
*/
func render(r *markdown.Renderer, body string) (rendered, error) {
	out, err := r.HTML([]byte(body))
	if err != nil {
		return rendered{}, fmt.Errorf("render article: %w", err)
	}
	toc, err := json.Marshal(tocOf(out.TOC))
	if err != nil {
		return rendered{}, fmt.Errorf("encode table of contents: %w", err)
	}
	return rendered{html: out.HTML, toc: toc}, nil
}

/*
TOCEntry is one heading of an article's table of contents.
*/
type TOCEntry struct {
	Level  int    `json:"level"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
}

/*
tocOf flattens the headings to two levels, the depth a reader scans.
*/
func tocOf(items []markdown.TOCItem) []TOCEntry {
	out := []TOCEntry{}
	for _, h2 := range items {
		if h2.ID != "" {
			out = append(out, TOCEntry{Level: 2, Text: h2.Title, Anchor: h2.ID})
		}
		for _, h3 := range h2.Items {
			if h3.ID != "" {
				out = append(out, TOCEntry{Level: 3, Text: h3.Title, Anchor: h3.ID})
			}
		}
	}
	return out
}

/*
draft is the reader-visible content of an article's working copy.
*/
type draft struct {
	title       string
	slug        string
	description string
	body        string
	visibility  string
	collection  *ids.UUID
}

/*
hash is the content hash of what a reader could see.
*/
func (d *draft) hash() string {
	collection := ""
	if d.collection != nil {
		collection = d.collection.String()
	}
	parts := []string{d.title, d.slug, d.description, d.body, d.visibility, collection}
	return hashing.SHA256Hex([]byte(strings.Join(parts, "\x00")))
}

/*
wordCount counts the words of a body, for reading time. Bodies are far
smaller than the largest count, which caps it all the same.
*/
func wordCount(body string) int32 {
	n := len(strings.FieldsFunc(markdown.ReadableText([]byte(body)), unicode.IsSpace))
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(n)
}

/*
slugPattern is what a slug is made of.
*/
func slugPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
}

/*
Slugify turns a title into a slug: lowercase letters and digits joined
by single dashes, accents transliterated, at most limit characters.
*/
func Slugify(title string, limit int) string {
	out := strings.Trim(slugChars().ReplaceAllString(str.Slug(title), "-"), "-")
	if len(out) > limit {
		out = strings.Trim(out[:limit], "-")
	}
	return out
}

/*
slugChars matches what a slug may not hold.
*/
func slugChars() *regexp.Regexp {
	return regexp.MustCompile(`[^a-z0-9]+`)
}
