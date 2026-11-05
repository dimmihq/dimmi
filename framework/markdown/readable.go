/*
|--------------------------------------------------------------------------
| Readable Text
|--------------------------------------------------------------------------
|
| A document as a person reads it where no formatting shows, such as the
| plain part of an email: the same lines the writer typed, blank lines
| between blocks, list items behind their bullets or numbers, quotes
| behind ">", and every link followed by where it goes, so nothing the
| formatted version offers is lost. Raw HTML reads as the words it is.
|
*/

package markdown

import (
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

/*
ReadableText returns Markdown source in the GFM dialect as plain text a
person reads without formatting.
*/
func ReadableText(src []byte) string {
	r := readable{src: src}
	r.blocks(parseGFM(src), "")

	return strings.TrimRight(r.out.String(), "\n")
}

/*
readable writes the text of a document into out.
*/
type readable struct {
	src []byte
	out strings.Builder
}

/*
blocks writes every block under n, each line behind the prefix.
*/
func (r *readable) blocks(n ast.Node, prefix string) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		r.block(c, prefix)
	}
}

/*
block writes one block and the blank line after it.
*/
func (r *readable) block(n ast.Node, prefix string) {
	switch n := n.(type) {
	case *ast.List:
		r.list(n, prefix)
	case *ast.Blockquote:
		inner := readable{src: r.src}
		inner.blocks(n, "")
		r.lines(prefix+"> ", strings.TrimRight(inner.out.String(), "\n"))
		r.trimQuoteEnd(prefix)
	case *ast.ThematicBreak:
		r.lines(prefix, "---")
	case *ast.Paragraph, *ast.Heading, *ast.TextBlock:
		r.lines(prefix, r.inline(n))
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		r.lines(prefix, strings.TrimRight(linesText(n, r.src), "\r\n"))
	case *extast.TableRow, *extast.TableHeader:
		r.row(n, prefix)
	case *ast.HTMLBlock:
		r.lines(prefix, htmlBlockText(n, r.src))
	default:
		r.blocks(n, prefix)
	}
}

/*
row writes a table row on one line, its cells apart.
*/
func (r *readable) row(n ast.Node, prefix string) {
	cells := make([]string, 0, n.ChildCount())
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		cells = append(cells, r.inline(c))
	}

	r.out.WriteString(prefix + strings.Join(cells, " | ") + "\n")

	if n.NextSibling() == nil {
		r.out.WriteString(strings.TrimRight(prefix, " ") + "\n")
	}
}

/*
list writes each item behind its bullet, or its number counting on from
where the list starts, its further lines indented under it.
*/
func (r *readable) list(l *ast.List, prefix string) {
	number := l.Start

	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		marker := "- "
		if l.IsOrdered() {
			marker = strconv.Itoa(number) + ". "
			number++
		}

		inner := readable{src: r.src}
		inner.blocks(item, "")

		lead, indent := marker, strings.Repeat(" ", len(marker))
		for line := range strings.SplitSeq(strings.TrimRight(inner.out.String(), "\n"), "\n") {
			r.out.WriteString(strings.TrimRight(prefix+lead+line, " ") + "\n")
			lead = indent
		}
	}

	r.out.WriteString(strings.TrimRight(prefix, " ") + "\n")
}

/*
trimQuoteEnd turns the blank line closing a quote into one outside it.
*/
func (r *readable) trimQuoteEnd(prefix string) {
	text := strings.TrimSuffix(r.out.String(), strings.TrimRight(prefix+"> ", " ")+"\n")
	r.out.Reset()
	r.out.WriteString(text + strings.TrimRight(prefix, " ") + "\n")
}

/*
lines writes text behind the prefix, line by line, then a blank line.
*/
func (r *readable) lines(prefix, text string) {
	for line := range strings.SplitSeq(text, "\n") {
		r.out.WriteString(strings.TrimRight(prefix+line, " ") + "\n")
	}

	r.out.WriteString(strings.TrimRight(prefix, " ") + "\n")
}

/*
inline returns the text of the inline content of n, keeping its line
breaks.
*/
func (r *readable) inline(n ast.Node) string {
	var out strings.Builder

	r.writeInline(&out, n)

	return strings.TrimSpace(out.String())
}

/*
writeInline writes inline content, each link followed by its address
unless its text already is the address.
*/
func (r *readable) writeInline(out *strings.Builder, n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c := c.(type) {
		case *ast.Text:
			out.Write(unescape(c.Segment.Value(r.src)))

			if c.SoftLineBreak() || c.HardLineBreak() {
				out.WriteByte('\n')
			}
		case *ast.Link:
			r.link(out, c)
		case *extast.TaskCheckBox:
			out.WriteString(map[bool]string{true: "[x] ", false: "[ ] "}[c.IsChecked])
		case *ast.String:
			out.Write(c.Value)
		case *ast.AutoLink:
			out.Write(c.Label(r.src))
		case *ast.CodeSpan:
			writeCode(out, c, r.src)
		case *ast.RawHTML:
			out.WriteString(rawHTMLText(c, r.src))
		default:
			r.writeInline(out, c)
		}
	}
}

/*
link writes a link's text and, after it, where the link goes.
*/
func (r *readable) link(out *strings.Builder, l *ast.Link) {
	var label strings.Builder

	r.writeInline(&label, l)

	destination := string(l.Destination)
	out.WriteString(label.String())

	if label.String() != destination && "mailto:"+label.String() != destination {
		out.WriteString(" (" + destination + ")")
	}
}
