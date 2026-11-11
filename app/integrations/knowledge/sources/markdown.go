/*
|--------------------------------------------------------------------------
| Markdown Helpers
|--------------------------------------------------------------------------
|
| Most sources hand over either HTML or Markdown files, so drivers share
| how both become the clean Markdown the index reads. HTML keeps its
| headings, lists, tables and links, and its pictures are collected so
| an import can copy them. A Markdown file's front matter gives its
| title and says whether it is a draft; MDX loses its imports, exports
| and component tags but keeps the text inside them.
|
*/

package sources

import (
	"path"
	"regexp"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/framework/dom"
	"github.com/dimmihq/dimmi/framework/markdown"
)

/*
maxFrontMatter is the largest front matter block read.
*/
const maxFrontMatter = 16 << 10

/*
HTMLToMarkdown converts an HTML body to Markdown and lists the pictures
it shows, in order, each once.
*/
func HTMLToMarkdown(body string) (string, []Asset, error) {
	md, err := dom.ToMarkdown(body)
	if err != nil {
		return "", nil, err
	}

	return Tidy(md), Images(body), nil
}

/*
Images lists the sources of the pictures an HTML body shows.
*/
func Images(body string) []Asset {
	doc, err := dom.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}

	var out []Asset

	seen := map[string]bool{}

	doc.Find("img").Each(func(s dom.Selection) {
		src, ok := s.Attr("src")
		if !ok || src == "" || seen[src] || strings.HasPrefix(src, "data:") {
			return
		}

		seen[src] = true
		alt, _ := s.Attr("alt")
		out = append(out, Asset{URL: src, Name: alt})
	})

	return out
}

/*
blankRuns finds three or more line breaks in a row.
*/
var blankRuns = regexp.MustCompile(`\n{3,}`)

/*
Tidy normalizes Markdown text: unified line breaks, no zero-width
characters, at most one blank line in a row, trimmed.
*/
func Tidy(md string) string {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	md = strings.Map(func(r rune) rune {
		if r == 0x200b || r == 0xfeff || r == 0x200c {
			return -1
		}

		return r
	}, md)

	return strings.TrimSpace(blankRuns.ReplaceAllString(md, "\n\n"))
}

/*
mdxLine matches the import and export lines of an MDX file.
*/
var mdxLine = regexp.MustCompile(`(?m)^(import|export)\s.*$`)

/*
mdxTag matches a JSX component tag, whose inner text is kept.
*/
var mdxTag = regexp.MustCompile(`</?[A-Z][A-Za-z0-9.]*(\s[^<>]*)?/?>`)

/*
mdxExpression matches a JSX expression in braces.
*/
var mdxExpression = regexp.MustCompile(`\{[^{}\n]*\}`)

/*
MarkdownFile reads a Markdown or MDX file at the path into a document:
its title from the front matter, else its first top heading, else its
file name; a draft or hidden flag in the front matter marks it a draft.
*/
func MarkdownFile(filePath string, src []byte) Document {
	meta, body, err := markdown.FrontMatter(src, maxFrontMatter)
	if err != nil {
		meta, body = nil, src
	}

	text := string(body)
	if strings.HasSuffix(strings.ToLower(filePath), ".mdx") {
		text = mdxExpression.ReplaceAllString(mdxTag.ReplaceAllString(mdxLine.ReplaceAllString(text, ""), ""), "")
	}

	doc := Document{Markdown: Tidy(text), Path: filePath}
	doc.Title = stringField(meta, "title")
	doc.Description = stringField(meta, "description")
	doc.Draft = boolField(meta, "draft") || boolField(meta, "hidden")

	if doc.Title == "" {
		doc.Title, doc.Markdown = firstHeading(doc.Markdown)
	}

	if doc.Title == "" {
		doc.Title = Humanize(filePath)
	}

	return doc
}

/*
firstHeading takes a leading top heading off the text as its title.
*/
func firstHeading(md string) (string, string) {
	first, rest, _ := strings.Cut(md, "\n")
	if title, ok := strings.CutPrefix(first, "# "); ok {
		return strings.TrimSpace(title), strings.TrimSpace(rest)
	}

	return "", md
}

/*
Humanize turns a file path into a title: "getting-started.md" reads
"Getting started".
*/
func Humanize(filePath string) string {
	name := strings.TrimSuffix(path.Base(filePath), path.Ext(filePath))
	name = strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(name))

	if name == "" {
		return "Untitled"
	}

	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])

	return string(runes)
}

/*
Folders turns the directories of a file path into collections,
outermost first.
*/
func Folders(filePath string) []Collection {
	dir := strings.Trim(path.Dir(filePath), "/.")
	if dir == "" {
		return nil
	}

	var out []Collection

	parts := strings.Split(dir, "/")
	for i, part := range parts {
		out = append(out, Collection{ExternalID: strings.Join(parts[:i+1], "/"), Name: Humanize(part)})
	}

	return out
}

func stringField(meta map[string]any, key string) string {
	s, _ := meta[key].(string)

	return strings.TrimSpace(s)
}

func boolField(meta map[string]any, key string) bool {
	b, _ := meta[key].(bool)

	return b
}
