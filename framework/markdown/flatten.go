/*
|--------------------------------------------------------------------------
| Flattened Blocks
|--------------------------------------------------------------------------
|
| The article blocks rewritten as plain Markdown, for readers that see
| only text, such as a language model: an embed and a button become a
| link to where they go, and a details block becomes its summary in bold
| followed by what it holds. Centered text is just text. Everything else stays exactly as written.
|
*/

package markdown

import (
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

/*
FlattenBlocks returns article Markdown with its embeds, buttons and
details blocks written as plain Markdown.
*/
func FlattenBlocks(src []byte) string {
	var edits []edit
	_ = ast.Walk(parseArticleDialect(src), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if d, ok := n.(*directiveNode); ok && entering {
			edits = append(edits, flatEdits(d)...)
		}
		return ast.WalkContinue, nil
	})
	slices.SortFunc(edits, func(a, b edit) int { return a.at.Start - b.at.Start })

	var out strings.Builder
	at := 0
	for _, e := range edits {
		start, stop := lineContent(src, e.at)
		if start < at {
			continue
		}
		out.Write(src[at:start])
		out.WriteString(e.with)
		at = stop
	}
	out.Write(src[at:])

	return out.String()
}

/*
edit replaces one line of the source.
*/
type edit struct {
	at   text.Segment
	with string
}

/*
flatEdits are the lines a directive rewrites.
*/
func flatEdits(d *directiveNode) []edit {
	switch d.name {
	case DirectiveEmbed:
		return []edit{{at: d.open, with: flatLink(d.label, d.attr("src"), "Video")}}
	case DirectiveButton:
		return []edit{{at: d.open, with: flatLink(d.label, d.attr("href"), "")}}
	}
	summary := strings.TrimSpace(d.label)
	if summary == "" {
		summary = "Details"
	}
	opening := "**" + escapeInline(summary) + "**"
	if d.name == DirectiveCenter {
		opening = ""
	}
	edits := []edit{{at: d.open, with: opening}}
	if d.close.Stop > d.close.Start {
		edits = append(edits, edit{at: d.close, with: ""})
	}
	return edits
}

/*
lineContent is where a line's text starts and stops, leaving out its
indent and line ending.
*/
func lineContent(src []byte, line text.Segment) (int, int) {
	start, stop := line.Start, line.Stop
	for stop > start && (src[stop-1] == '\n' || src[stop-1] == '\r') {
		stop--
	}
	for start < stop && (src[start] == ' ' || src[start] == '\t') {
		start++
	}
	return start, stop
}

/*
flatLink writes a label and an address as a Markdown link, the label
standing in for itself when there is no address and the address for
itself when there is no label.
*/
func flatLink(label, href, fallback string) string {
	label = strings.TrimSpace(label)
	href = strings.TrimSpace(href)
	switch {
	case href == "" && label == "":
		return fallback
	case href == "":
		return escapeInline(label)
	case label == "":
		label = fallback
	}
	if label == "" {
		return "<" + href + ">"
	}
	return "[" + escapeInline(label) + "](<" + strings.ReplaceAll(href, ">", "%3E") + ">)"
}

/*
escapeInline escapes the characters that would read as Markdown inside
a label.
*/
func escapeInline(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\[]*_<>`+"`", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
