/*
|--------------------------------------------------------------------------
| Markdown Renderer
|--------------------------------------------------------------------------
|
| Markdown is the stored form of everything people and models write, so
| rendering it is where untrusted text becomes HTML. Raw HTML inside the
| Markdown is shown as text and never rendered, dangerous URLs are
| dropped, and the finished HTML always passes through a sanitize policy
| as a second line of defense.
|
| A renderer is built once from its options and is safe to share. Each
| render also returns the table of contents and the warnings an editor
| should see before publishing.
|
*/

package markdown

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/toc"

	"github.com/dimmihq/dimmi/framework/sanitize"
)

/*
Priorities of this package's additions to the Markdown engine. They run
ahead of the engine's own transformers and renderers, which use larger
numbers.
*/
const (
	alertPriority    = 100
	rendererPriority = 100
)

/*
Options choose the Markdown dialect and how it renders.

GFM adds tables, strikethrough, task lists and bare URLs as links.
Footnotes adds GFM footnotes. Alerts renders GitHub alerts such as
"> [!NOTE]" as aside elements. HeadingIDs gives every heading a unique
id, keeping letters of every script. Highlight colors fenced code
blocks of a known language with CSS classes; StyleSheet returns the
matching CSS. HardWraps keeps every line break inside a paragraph, as a
person typing into a message box expects.

RewriteLink, when set, sees every link and image and returns what to
render instead. Sanitize is the policy the HTML passes through; nil
means Policy().
*/
type Options struct {
	GFM         bool
	Footnotes   bool
	Alerts      bool
	HeadingIDs  bool
	Highlight   bool
	HardWraps   bool
	RewriteLink func(Link) Link
	Sanitize    *sanitize.Policy
}

/*
Result is a rendered document: the sanitized HTML, the table of
contents built from its headings, and the warnings found on the way.
*/
type Result struct {
	HTML     string
	TOC      []TOCItem
	Warnings []Warning
}

/*
TOCItem is a heading in the table of contents, with the headings nested
under it. ID is empty unless the renderer gives headings ids.
*/
type TOCItem struct {
	Title string
	ID    string
	Items []TOCItem
}

/*
Renderer turns Markdown into HTML with one fixed set of options. It is
safe for concurrent use.
*/
type Renderer struct {
	md      goldmark.Markdown
	rewrite func(Link) Link
	policy  *sanitize.Policy
}

/*
New returns a renderer for the options.
*/
func New(opts Options) *Renderer {
	policy := opts.Sanitize
	if policy == nil {
		policy = Policy()
	}

	return &Renderer{
		md:      newEngine(opts),
		rewrite: opts.RewriteLink,
		policy:  policy,
	}
}

/*
HTML renders Markdown source to sanitized HTML.
*/
func (r *Renderer) HTML(src []byte) (Result, error) {
	ctx := parser.NewContext(parser.WithIDs(newHeadingIDs()))
	doc := r.md.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))

	warnings := inspect(doc, src, r.rewrite)

	contents, err := toc.Inspect(doc, src, toc.Compact(true))
	if err != nil {
		return Result{}, fmt.Errorf("build table of contents: %w", err)
	}

	var out bytes.Buffer
	if err := r.md.Renderer().Render(&out, src, doc); err != nil {
		return Result{}, fmt.Errorf("render markdown: %w", err)
	}

	return Result{
		HTML:     r.policy.Sanitize(out.String()),
		TOC:      tocItems(contents.Items),
		Warnings: warnings,
	}, nil
}

/*
newEngine builds the Markdown engine for the options.
*/
func newEngine(opts Options) goldmark.Markdown {
	var (
		extensions []goldmark.Extender
		parsing    []parser.Option
	)

	if opts.GFM {
		extensions = append(extensions, extension.GFM)
	}

	if opts.Footnotes {
		extensions = append(extensions, extension.Footnote)
	}

	if opts.Alerts {
		parsing = append(parsing, parser.WithASTTransformers(util.Prioritized(alertTransformer{}, alertPriority)))
	}

	if opts.HeadingIDs {
		parsing = append(parsing, parser.WithAutoHeadingID())
	}

	nodes := newNodeRenderer(opts.Highlight, opts.Alerts)
	rendering := []renderer.Option{renderer.WithNodeRenderers(util.Prioritized(nodes, rendererPriority))}

	if opts.HardWraps {
		rendering = append(rendering, html.WithHardWraps())
	}

	return goldmark.New(
		goldmark.WithExtensions(extensions...),
		goldmark.WithParserOptions(parsing...),
		goldmark.WithRendererOptions(rendering...),
	)
}

/*
tocItems copies the table of contents out of the library's types.
*/
func tocItems(items toc.Items) []TOCItem {
	if len(items) == 0 {
		return nil
	}

	out := make([]TOCItem, len(items))
	for i, item := range items {
		out[i] = TOCItem{Title: string(item.Title), ID: string(item.ID), Items: tocItems(item.Items)}
	}

	return out
}
