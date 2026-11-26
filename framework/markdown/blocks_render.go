/*
|--------------------------------------------------------------------------
| Block Output
|--------------------------------------------------------------------------
|
| The HTML of the article blocks. An embed is a frame from an allowed
| provider's player, loaded lazily, titled for screen readers, sandboxed
| and sent no more than the page's origin; an address that is no embed
| is a link instead. A button is a link drawn as a button. A details
| block is the browser's own disclosure, so it opens with the keyboard
| and reads correctly with no script. A figure is a picture with its
| caption.
|
*/

package markdown

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

/*
Fixed attributes of every embed frame.
*/
const (
	embedAllow    = "autoplay; fullscreen; picture-in-picture; encrypted-media; clipboard-write"
	embedSandbox  = "allow-scripts allow-same-origin allow-popups allow-presentation allow-forms"
	embedReferrer = "strict-origin-when-cross-origin"
)

/*
renderDirective writes a directive.
*/
func (r *nodeRenderer) renderDirective(
	w util.BufWriter, _ []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n, ok := node.(*directiveNode)

	switch {
	case !ok:
		return ast.WalkContinue, nil
	case n.name == DirectiveEmbed && entering:
		writeEmbed(w, n)
		return ast.WalkSkipChildren, nil
	case n.name == DirectiveButton && entering:
		return openButton(w, n), nil
	case n.name == DirectiveButton:
		closeButton(w, n)
	case n.name == DirectiveDetails && entering:
		openDetails(w, n)
	case n.name == DirectiveDetails:
		_, _ = w.WriteString("</details>\n")
	}
	return ast.WalkContinue, nil
}

/*
writeEmbed writes an embed's frame, or a link to its address when it is
not from an allowed provider.
*/
func writeEmbed(w util.BufWriter, n *directiveNode) {
	src := n.attr("src")
	title := strings.TrimSpace(n.label)
	embed, ok := ResolveEmbed(src)
	if !ok {
		writeFallbackLink(w, src, title)
		return
	}
	if title == "" {
		title = "Embedded content"
	}

	_, _ = w.WriteString(`<figure class="embed embed-` + embed.Provider + `"><iframe src="`)
	_, _ = w.Write(util.EscapeHTML([]byte(embed.Player)))
	_, _ = w.WriteString(`" title="`)
	_, _ = w.Write(util.EscapeHTML([]byte(title)))
	_, _ = w.WriteString(`" loading="lazy" allow="` + embedAllow + `" allowfullscreen="" sandbox="` + embedSandbox +
		`" referrerpolicy="` + embedReferrer + `"></iframe></figure>` + "\n")
}

/*
writeFallbackLink writes an address that could not be embedded as a
link, when it is a web address, or only its label.
*/
func writeFallbackLink(w util.BufWriter, src, label string) {
	if label == "" {
		label = src
	}
	if label == "" {
		return
	}
	_, _ = w.WriteString("<p>")
	if isWebURL(src) {
		_, _ = w.WriteString(`<a href="`)
		_, _ = w.Write(util.EscapeHTML(util.URLEscape([]byte(src), true)))
		_, _ = w.WriteString(`" rel="` + externalRel + `">`)
		_, _ = w.Write(util.EscapeHTML([]byte(label)))
		_, _ = w.WriteString("</a>")
	} else {
		_, _ = w.Write(util.EscapeHTML([]byte(label)))
	}
	_, _ = w.WriteString("</p>\n")
}

/*
isWebURL reports whether an address is an absolute http or https one.
*/
func isWebURL(s string) bool {
	lower := strings.ToLower(s)
	return (strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")) && len(s) > len("https://")
}

/*
openButton opens a button's row; the link inside it is written by the
engine. A button without a link shows its label instead.
*/
func openButton(w util.BufWriter, n *directiveNode) ast.WalkStatus {
	if n.ChildCount() > 0 {
		_, _ = w.WriteString(`<p class="button-row button-align-` + buttonAlign(n) + `">`)
		return ast.WalkContinue
	}
	if label := strings.TrimSpace(n.label); label != "" {
		_, _ = w.WriteString("<p>")
		_, _ = w.Write(util.EscapeHTML([]byte(label)))
		_, _ = w.WriteString("</p>\n")
	}
	return ast.WalkSkipChildren
}

/*
closeButton closes a button's row.
*/
func closeButton(w util.BufWriter, n *directiveNode) {
	if n.ChildCount() > 0 {
		_, _ = w.WriteString("</p>\n")
	}
}

/*
openDetails opens a details block, its label the summary.
*/
func openDetails(w util.BufWriter, n *directiveNode) {
	summary := strings.TrimSpace(n.label)
	if summary == "" {
		summary = "Details"
	}
	_, _ = w.WriteString(`<details class="details"><summary>`)
	_, _ = w.Write(util.EscapeHTML([]byte(summary)))
	_, _ = w.WriteString("</summary>\n")
}

/*
renderFigure writes a captioned picture.
*/
func (r *nodeRenderer) renderFigure(
	w util.BufWriter, _ []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n, ok := node.(*figureNode)

	switch {
	case !ok:
	case entering:
		openFigure(w, n)
	default:
		_, _ = w.WriteString("<figcaption>")
		_, _ = w.Write(util.EscapeHTML([]byte(n.caption)))
		_, _ = w.WriteString("</figcaption></figure>\n")
	}
	return ast.WalkContinue, nil
}

/*
openFigure opens a figure, placed where its picture is.
*/
func openFigure(w util.BufWriter, n *figureNode) {
	class := "image-figure"
	if align := figureAlign(n); align != "" {
		class += " image-figure-" + align
	}
	_, _ = w.WriteString(`<figure class="` + class + `">`)
}

/*
figureAlign is where the figure's picture sits, from its layout class.
*/
func figureAlign(n *figureNode) string {
	img, ok := soleImageOf(n)
	if !ok {
		return ""
	}
	class, ok := img.AttributeString("class")
	if !ok {
		return ""
	}
	b, _ := class.([]byte)
	for c := range strings.FieldsSeq(string(b)) {
		if align, found := strings.CutPrefix(c, "image-align-"); found {
			return align
		}
	}
	return ""
}

/*
soleImageOf returns the picture a figure holds.
*/
func soleImageOf(n ast.Node) (*ast.Image, bool) {
	child := n.FirstChild()
	if link, ok := child.(*ast.Link); ok {
		child = link.FirstChild()
	}
	img, ok := child.(*ast.Image)
	return img, ok
}
