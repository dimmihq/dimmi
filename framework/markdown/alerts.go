/*
|--------------------------------------------------------------------------
| Alerts
|--------------------------------------------------------------------------
|
| GitHub's alert syntax: a quote whose first line is exactly one of
| [!NOTE], [!TIP], [!IMPORTANT], [!WARNING] or [!CAUTION]. The marker
| line is removed and the quote is tagged with its kind, so it renders
| as an aside with the kind in its class instead of a plain quote. A
| quote with any other first line stays a quote.
|
*/

package markdown

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

/*
alertAttribute is the attribute that tags a quote with its alert kind.
It only lives in the syntax tree; the renderer writes a class instead.
*/
const alertAttribute = "alert"

/*
alertTransformer turns quotes that start with an alert marker into
alerts.
*/
type alertTransformer struct{}

/*
Transform tags every alert quote in the document.
*/
func (alertTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()

	for _, quote := range quotes(doc, nil) {
		para, ok := quote.FirstChild().(*ast.Paragraph)
		if !ok || para.Lines().Len() == 0 {
			continue
		}

		first := para.Lines().At(0)

		kind, ok := alertKind(first.Value(src))
		if !ok {
			continue
		}

		dropFirstLine(para)
		quote.SetAttributeString(alertAttribute, []byte(kind))
	}
}

/*
quotes returns every block quote in the document.
*/
func quotes(n ast.Node, out []*ast.Blockquote) []*ast.Blockquote {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if quote, ok := c.(*ast.Blockquote); ok {
			out = append(out, quote)
		}

		out = quotes(c, out)
	}

	return out
}

/*
alertKind returns the lowercase kind a marker line names.
*/
func alertKind(line []byte) (string, bool) {
	marker := bytes.TrimSpace(line)
	if !bytes.HasPrefix(marker, []byte("[!")) || !bytes.HasSuffix(marker, []byte("]")) {
		return "", false
	}

	kind := strings.ToLower(string(marker[2 : len(marker)-1]))

	switch kind {
	case "note", "tip", "important", "warning", "caution":
		return kind, true
	default:
		return "", false
	}
}

/*
dropFirstLine removes the marker line from a paragraph, and the
paragraph itself when nothing follows the marker.
*/
func dropFirstLine(para *ast.Paragraph) {
	lines := para.Lines()
	end := lines.At(0).Stop

	for c := para.FirstChild(); c != nil; c = para.FirstChild() {
		t, ok := c.(*ast.Text)
		if !ok || t.Segment.Stop > end {
			break
		}

		para.RemoveChild(para, c)
	}

	lines.SetSliced(1, lines.Len())

	if para.ChildCount() == 0 {
		para.Parent().RemoveChild(para.Parent(), para)
	}
}

/*
alertOf returns the alert kind a quote was tagged with.
*/
func alertOf(n ast.Node) (string, bool) {
	kind, ok := n.AttributeString(alertAttribute)
	if !ok {
		return "", false
	}

	b, ok := kind.([]byte)

	return string(b), ok
}
