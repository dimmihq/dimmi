/*
|--------------------------------------------------------------------------
| Plain Text
|--------------------------------------------------------------------------
|
| The words of a document without its markup, for search indexes, word
| counts, excerpts and prompts. Each paragraph, heading, list item, table
| row and code block becomes one line. Links keep their text, images
| their alt text, and escapes and entities are resolved. Raw HTML counts
| as the words it is, the same way the renderer shows it as text.
|
*/

package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

/*
PlainText returns the text of Markdown source in the GFM dialect with
footnotes and alerts. Footnote texts follow the document.
*/
func PlainText(src []byte) string {
	var lines []string

	collectText(parseGFM(src), src, &lines)

	return strings.Join(lines, "\n")
}

/*
parseGFM reads Markdown in the GFM dialect with footnotes and alerts,
the dialect the package-level functions share.
*/
func parseGFM(src []byte) ast.Node {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote),
		goldmark.WithParserOptions(parser.WithASTTransformers(util.Prioritized(alertTransformer{}, alertPriority))),
	)

	return md.Parser().Parse(text.NewReader(src))
}

/*
collectText appends one line per text block under n.
*/
func collectText(n ast.Node, src []byte, lines *[]string) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		line, isBlock := blockText(c, src)
		if !isBlock {
			collectText(c, src, lines)

			continue
		}

		if line = strings.TrimSpace(line); line != "" {
			*lines = append(*lines, line)
		}
	}
}

/*
blockText returns the text of a block that holds text directly, and
false for containers whose children hold it.
*/
func blockText(n ast.Node, src []byte) (string, bool) {
	switch n := n.(type) {
	case *ast.Paragraph, *ast.Heading, *ast.TextBlock:
		return inlineText(n, src), true
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		return linesText(n, src), true
	case *extast.TableRow, *extast.TableHeader:
		return rowText(n, src), true
	case *ast.HTMLBlock:
		return htmlBlockText(n, src), true
	default:
		return "", false
	}
}

/*
rowText returns the text of a table row, its cells joined by spaces.
*/
func rowText(row ast.Node, src []byte) string {
	cells := make([]string, 0, row.ChildCount())
	for c := row.FirstChild(); c != nil; c = c.NextSibling() {
		cells = append(cells, inlineText(c, src))
	}

	return strings.Join(cells, " ")
}

/*
inlineText returns the text of the inline content of n, with line breaks
as spaces.
*/
func inlineText(n ast.Node, src []byte) string {
	var out strings.Builder

	writeInline(&out, n, src)

	return out.String()
}

/*
writeInline writes the text of the inline content of n.
*/
func writeInline(out *strings.Builder, n ast.Node, src []byte) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Text:
			out.Write(unescape(c.Segment.Value(src)))

			if c.SoftLineBreak() || c.HardLineBreak() {
				out.WriteByte(' ')
			}
		case *ast.String:
			out.Write(c.Value)
		case *ast.AutoLink:
			out.Write(c.Label(src))
		case *ast.CodeSpan:
			writeCode(out, c, src)
		case *ast.RawHTML:
			out.WriteString(rawHTMLText(c, src))
		default:
			writeInline(out, c, src)
		}
	}
}

/*
writeCode writes the text of a code span exactly as written, since
escapes and entities mean nothing inside code.
*/
func writeCode(out *strings.Builder, n *ast.CodeSpan, src []byte) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			out.Write(t.Segment.Value(src))
		}
	}
}

/*
linesText returns the raw lines of a block, such as a code block.
*/
func linesText(n ast.Node, src []byte) string {
	return string(n.Lines().Value(src))
}

/*
unescape resolves entity references and backslash escapes in text.
*/
func unescape(b []byte) []byte {
	return util.UnescapePunctuations(util.ResolveNumericReferences(util.ResolveEntityNames(b)))
}
