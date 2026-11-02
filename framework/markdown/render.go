/*
|--------------------------------------------------------------------------
| HTML Output
|--------------------------------------------------------------------------
|
| The few places where this package writes HTML itself instead of the
| Markdown engine. Raw HTML in the source is written as escaped text, so
| what the author typed is visible and nothing in it runs. Alerts render
| as asides. Fenced code of a known language is split into tokens with
| CSS classes, never inline styles, so a strict content security policy
| and a sanitizer can both allow it; unknown languages stay plain.
|
| The token classes all start with "chroma-". StyleSheet returns CSS for
| them in any of the highlighter's named styles.
|
*/

package markdown

import (
	"bytes"
	"fmt"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

/*
classPrefix starts every highlighting class.
*/
const classPrefix = "chroma-"

/*
nodeRenderer writes the HTML for raw HTML, alerts and highlighted code.
*/
type nodeRenderer struct {
	highlight bool
	alerts    bool
	formatter *chromahtml.Formatter
}

/*
newNodeRenderer returns the renderer for the enabled features.
*/
func newNodeRenderer(highlight, alerts bool) *nodeRenderer {
	return &nodeRenderer{highlight: highlight, alerts: alerts, formatter: newFormatter()}
}

/*
newFormatter returns the highlighter output settings shared by code
blocks and StyleSheet, so the classes and the CSS always agree.
*/
func newFormatter() *chromahtml.Formatter {
	return chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.WithAllClasses(true),
		chromahtml.ClassPrefix(classPrefix),
	)
}

/*
StyleSheet returns the CSS that colors highlighted code in one of the
highlighter's named styles, such as "github" or "monokai". An unknown
name gives the default style.
*/
func StyleSheet(style string) (string, error) {
	var css bytes.Buffer
	if err := newFormatter().WriteCSS(&css, styles.Get(style)); err != nil {
		return "", fmt.Errorf("write highlight css: %w", err)
	}

	return css.String(), nil
}

/*
RegisterFuncs registers the renderers for the enabled features.
*/
func (r *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindRawHTML, leaf(r.renderRawHTML))
	reg.Register(ast.KindHTMLBlock, leaf(r.renderHTMLBlock))

	if r.alerts {
		reg.Register(ast.KindBlockquote, r.renderBlockquote)
	}

	if r.highlight {
		reg.Register(ast.KindFencedCodeBlock, leaf(r.renderCode))
	}
}

/*
leaf adapts a function that writes a whole node at once, children
included, to the engine's enter and exit calls.
*/
func leaf(write func(w util.BufWriter, src []byte, node ast.Node)) renderer.NodeRendererFunc {
	return func(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			write(w, src, node)
		}

		return ast.WalkSkipChildren, nil
	}
}

/*
renderRawHTML writes inline HTML from the source as text.
*/
func (r *nodeRenderer) renderRawHTML(w util.BufWriter, src []byte, node ast.Node) {
	if raw, ok := node.(*ast.RawHTML); ok {
		_, _ = w.Write(util.EscapeHTML([]byte(rawHTMLText(raw, src))))
	}
}

/*
renderHTMLBlock writes a block of HTML from the source as a paragraph
of text.
*/
func (r *nodeRenderer) renderHTMLBlock(w util.BufWriter, src []byte, node ast.Node) {
	if block, ok := node.(*ast.HTMLBlock); ok {
		_, _ = w.WriteString("<p>")
		_, _ = w.Write(util.EscapeHTML([]byte(htmlBlockText(block, src))))
		_, _ = w.WriteString("</p>\n")
	}
}

/*
renderBlockquote writes alerts as asides and other quotes as quotes.
*/
func (r *nodeRenderer) renderBlockquote(
	w util.BufWriter, _ []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	kind, isAlert := alertOf(node)

	switch {
	case isAlert && entering:
		_, _ = fmt.Fprintf(w, "<aside class=\"alert alert-%s\" role=\"note\">\n", kind)
	case isAlert:
		_, _ = w.WriteString("</aside>\n")
	case entering:
		_, _ = w.WriteString("<blockquote>\n")
	default:
		_, _ = w.WriteString("</blockquote>\n")
	}

	return ast.WalkContinue, nil
}

/*
renderCode writes a fenced code block, highlighted when its language is
known and plain otherwise.
*/
func (r *nodeRenderer) renderCode(w util.BufWriter, src []byte, node ast.Node) {
	block, ok := node.(*ast.FencedCodeBlock)
	if !ok {
		return
	}

	code := linesText(block, src)
	if r.writeHighlighted(w, string(block.Language(src)), code) {
		return
	}

	_, _ = w.WriteString("<pre><code>")
	_, _ = w.Write(util.EscapeHTML([]byte(code)))
	_, _ = w.WriteString("</code></pre>\n")
}

/*
writeHighlighted writes highlighted code and reports whether it could:
the language must be known and the code must split into tokens.
*/
func (r *nodeRenderer) writeHighlighted(w util.BufWriter, language, code string) bool {
	if language == "" {
		return false
	}

	lexer := lexers.Get(language)
	if lexer == nil {
		return false
	}

	tokens, err := chroma.Coalesce(lexer).Tokenise(nil, code)
	if err != nil {
		return false
	}

	var out bytes.Buffer
	if err := r.formatter.Format(&out, styles.Fallback, tokens); err != nil {
		return false
	}

	_, _ = w.Write(out.Bytes())

	return true
}
