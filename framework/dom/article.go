/*
|--------------------------------------------------------------------------
| Article Conversion
|--------------------------------------------------------------------------
|
| Help center articles from other tools turned into dimmi's article
| Markdown, keeping the blocks plain Markdown has no syntax for. Each
| block is read from the HTML Intercom's Articles API returns:
|
|   - a button, <a class="intercom-h2b-button">, becomes ::button, centered
|     when it sits in <div class="intercom-align-center">
|   - a video, an <iframe>, becomes ::embed with its address
|   - a picture in <div class="intercom-align-center"> keeps its place
|   - a callout, <div class="intercom-interblocks-callout">, becomes the
|     alert its background color is closest to
|   - a collapsible section, <details> with a <summary>, becomes
|     :::details
|   - a <figure> keeps its <figcaption> as the picture's caption
|   - a paragraph or heading with class "intercom-align-center" goes into
|     a :::center block with its centered neighbors
|
| dimmi's own article HTML converts back the same way, so an article
| survives a round trip: its button rows, alert asides, embed frames and
| picture layout classes are read too.
|
| The button, video and alignment shapes are documented by Intercom. The
| callout's was seen in a live article and the collapsible section's is
| not documented at all; both stay unverified until a real export is
| read through this converter.
|
*/

package dom

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
)

/*
blankLines finds runs of blank lines to fold into one.
*/
var blankLines = regexp.MustCompile(`\n[ \t]*(?:\n[ \t]*)+\n`)

/*
Intercom's class names for the blocks it writes.
*/
const (
	intercomButton  = "intercom-h2b-button"
	intercomCenter  = "intercom-align-center"
	intercomCallout = "intercom-interblocks-callout"
)

/*
ToArticleMarkdown converts an article's HTML to article Markdown: what
ToMarkdown keeps, plus buttons, embeds, callouts, collapsible sections,
centered pictures and captions. Like ToMarkdown, the result is as
untrusted as the HTML; the article renderer checks every address.
*/
func ToArticleMarkdown(body string) (string, error) {
	convert := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(commonmark.WithListEndComment(false)),
		table.NewTablePlugin(
			table.WithCellPaddingBehavior(table.CellPaddingBehaviorMinimal),
			table.WithSkipEmptyRows(true),
			table.WithHeaderPromotion(true),
		),
		articlePlugin{},
	))

	out, err := convert.ConvertString(body)
	if err != nil {
		return "", fmt.Errorf("convert article html to markdown: %w", err)
	}

	return out, nil
}

/*
articlePlugin adds the article blocks to the converter.
*/
type articlePlugin struct{}

/*
Name names the plugin.
*/
func (articlePlugin) Name() string {
	return "dimmi-article-blocks"
}

/*
Init registers the blocks' renderers ahead of the standard ones.
*/
func (articlePlugin) Init(conv *converter.Converter) error {
	conv.Register.PreRenderer(preparePictures, converter.PriorityEarly)
	conv.Register.TagType("iframe", converter.TagTypeBlock, converter.PriorityEarly)
	conv.Register.TagType("details", converter.TagTypeBlock, converter.PriorityEarly)
	conv.Register.Renderer(renderArticleBlock, converter.PriorityEarly)

	return nil
}

/*
renderArticleBlock writes the blocks the plugin knows and leaves every
other element to the standard renderers.
*/
func renderArticleBlock(ctx converter.Context, w converter.Writer, n *html.Node) converter.RenderStatus {
	if n.Type != html.ElementNode {
		return converter.RenderTryNext
	}

	switch {
	case n.Data == "a" && (hasClass(n, intercomButton) || isOwnButton(n)):
		writeButton(w, n)
	case n.Data == "iframe":
		writeEmbed(w, n)
	case n.Data == "details":
		writeDetails(ctx, w, n)
	case n.Data == "div" && hasClass(n, "align-center"):
		writeCenter(ctx, w, n)
	case n.Data == "div" && hasClass(n, intercomCallout):
		writeCallout(ctx, w, n, calloutKind(styleOf(n)))
	case n.Data == "aside" && ownAlert(n) != "":
		writeCallout(ctx, w, n, ownAlert(n))
	default:
		return converter.RenderTryNext
	}

	return converter.RenderSuccess
}

/*
writeButton writes a button, centered when Intercom centered it.
*/
func writeButton(w converter.Writer, n *html.Node) {
	href, _ := attr(n, "href")
	label := collapse(textOf(n))
	attrs := `href=` + quoteAttr(strings.TrimSpace(href))
	switch p := n.Parent; {
	case p == nil:
	case hasClass(p, intercomCenter) || hasClass(p, "button-align-center"):
		attrs += ` align="center"`
	case hasClass(p, "button-align-end"):
		attrs += ` align="end"`
	}
	block(w, "::button["+escapeLabel(label)+"]{"+attrs+"}")
}

/*
isOwnButton reports whether a link is a button dimmi drew.
*/
func isOwnButton(n *html.Node) bool {
	return hasClass(n, "button") && n.Parent != nil && hasClass(n.Parent, "button-row")
}

/*
ownAlert is the kind of an alert aside dimmi drew, in capitals.
*/
func ownAlert(n *html.Node) string {
	if !hasClass(n, "alert") {
		return ""
	}
	for _, kind := range []string{"note", "tip", "important", "warning", "caution"} {
		if hasClass(n, "alert-"+kind) {
			return strings.ToUpper(kind)
		}
	}
	return ""
}

func styleOf(n *html.Node) string {
	style, _ := attr(n, "style")
	return style
}

/*
writeEmbed writes a frame as an embed of its address. An address from a
provider dimmi does not embed still converts; the renderer shows it as
a link and warns the editor.
*/
func writeEmbed(w converter.Writer, n *html.Node) {
	src, _ := attr(n, "src")
	src = strings.TrimSpace(src)
	if src == "" {
		return
	}
	if strings.HasPrefix(src, "//") {
		src = "https:" + src
	}
	title, _ := attr(n, "title")
	block(w, "::embed["+escapeLabel(collapse(title))+"]{src="+quoteAttr(src)+"}")
}

/*
writeDetails writes a collapsible section around its converted content,
with more colons than any section inside it.
*/
func writeDetails(ctx converter.Context, w converter.Writer, n *html.Node) {
	var summary string
	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "summary" && summary == "" {
			summary = collapse(textOf(c))
			continue
		}
		ctx.RenderNodes(ctx, &buf, c)
	}
	fence := strings.Repeat(":", 3+containerDepth(n))
	content := bytes.TrimSpace(buf.Bytes())
	out := fence + "details[" + escapeLabel(summary) + "]\n"
	if len(content) > 0 {
		out += string(content) + "\n"
	}
	block(w, out+fence)
}

/*
writeCenter writes centered blocks as a center block.
*/
func writeCenter(ctx converter.Context, w converter.Writer, n *html.Node) {
	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := bytes.TrimSpace(buf.Bytes())
	if len(content) == 0 {
		return
	}
	fence := strings.Repeat(":", 3+containerDepth(n))
	block(w, fence+"center\n"+string(content)+"\n"+fence)
}

/*
containerDepth counts how deeply sections and center blocks nest inside
an element, so its fence can hold them all.
*/
func containerDepth(n *html.Node) int {
	deepest := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		d := containerDepth(c)
		if isContainer(c) {
			d++
		}
		deepest = max(deepest, d)
	}
	return deepest
}

/*
isContainer reports whether an element converts to a container block.
*/
func isContainer(n *html.Node) bool {
	return n.Type == html.ElementNode && (n.Data == "details" || (n.Data == "div" && hasClass(n, "align-center")))
}

/*
writeCallout writes a callout as an alert of the kind.
*/
func writeCallout(ctx converter.Context, w converter.Writer, n *html.Node, kind string) {
	var buf bytes.Buffer
	ctx.RenderChildNodes(ctx, &buf, n)
	content := blankLines.ReplaceAll(bytes.TrimSpace(buf.Bytes()), []byte("\n\n"))

	lines := []string{"> [!" + kind + "]"}
	for line := range strings.SplitSeq(string(content), "\n") {
		if strings.TrimSpace(line) == "" {
			lines = append(lines, ">")
			continue
		}
		lines = append(lines, "> "+line)
	}
	block(w, strings.Join(lines, "\n"))
}

/*
calloutKind picks the alert for a callout's background color: yellow and
orange warn, red cautions, green is a tip, purple is important, and
blue, grey or no color is a note.
*/
func calloutKind(style string) string {
	hue, saturated, ok := backgroundHue(style)
	switch {
	case !ok || !saturated:
		return "NOTE"
	case hue < 20 || hue >= 330:
		return "CAUTION"
	case hue < 70:
		return "WARNING"
	case hue < 170:
		return "TIP"
	case hue < 260:
		return "NOTE"
	default:
		return "IMPORTANT"
	}
}

/*
backgroundHue reads the hue of a hex background-color in a style, and
whether the color is saturated enough to have one.
*/
func backgroundHue(style string) (float64, bool, bool) {
	for decl := range strings.SplitSeq(style, ";") {
		name, value, found := strings.Cut(decl, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "background-color") {
			continue
		}
		rgb, ok := hexColor(strings.TrimSpace(value))
		if !ok {
			return 0, false, false
		}
		r, g, b := rgb[0], rgb[1], rgb[2]
		hi, lo := max(r, g, b), min(r, g, b)
		if hi-lo < 0.08 {
			return 0, false, true
		}
		var hue float64
		switch hi {
		case r:
			hue = 60 * (g - b) / (hi - lo)
		case g:
			hue = 60 * (2 + (b-r)/(hi-lo))
		default:
			hue = 60 * (4 + (r-g)/(hi-lo))
		}
		if hue < 0 {
			hue += 360
		}
		return hue, true, true
	}
	return 0, false, false
}

/*
hexColor reads #rgb, #rrggbb and #rrggbbaa as fractions of one.
*/
func hexColor(s string) ([3]float64, bool) {
	var rgb [3]float64
	s, ok := strings.CutPrefix(s, "#")
	if !ok {
		return rgb, false
	}
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 && len(s) != 8 {
		return rgb, false
	}
	for i := range rgb {
		v, err := strconv.ParseUint(s[i*2:i*2+2], 16, 8)
		if err != nil {
			return rgb, false
		}
		rgb[i] = float64(v) / 255
	}
	return rgb, true
}

/*
preparePictures moves what Intercom says about a picture into the
picture itself before conversion: its place in the column into its
address's layout fragment, and a figure's caption into its title.
*/
func preparePictures(_ converter.Context, doc *html.Node) {
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "img":
			if src, ok := attr(n, "src"); ok && !strings.Contains(src, "#") {
				setAttr(n, "src", src+layoutFragment(n))
			}
		case "figure":
			caption := findChild(n, "figcaption")
			img := findDescendant(n, "img")
			if caption != nil && img != nil {
				setAttr(img, "title", collapse(textOf(caption)))
				n.RemoveChild(caption)
			}
		}
	}
	walk(doc)
	gatherCentered(doc)
}

/*
gatherCentered wraps each run of paragraphs and headings Intercom
centered in one center block, the way dimmi writes centered text.
*/
func gatherCentered(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		gatherCentered(c)
	}
	var wrapper *html.Node
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		switch {
		case isCenteredText(c):
			if wrapper == nil {
				wrapper = &html.Node{
					Type: html.ElementNode, Data: "div",
					Attr: []html.Attribute{{Key: "class", Val: "align-center"}},
				}
				n.InsertBefore(wrapper, c)
			}
			n.RemoveChild(c)
			wrapper.AppendChild(c)
		case c.Type == html.TextNode && strings.TrimSpace(c.Data) == "":
		default:
			wrapper = nil
		}
		c = next
	}
}

/*
isCenteredText reports whether an element is a paragraph or heading
Intercom centered.
*/
func isCenteredText(n *html.Node) bool {
	if n.Type != html.ElementNode || !hasClass(n, intercomCenter) {
		return false
	}
	switch n.Data {
	case "p", "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}
	return false
}

/*
layoutFragment is the layout fragment for a picture's width and place:
Intercom's centering wrapper, or dimmi's own layout classes.
*/
func layoutFragment(n *html.Node) string {
	var parts []string
	class, _ := attr(n, "class")
	for c := range strings.FieldsSeq(class) {
		if width, ok := strings.CutPrefix(c, "image-width-"); ok {
			if _, err := strconv.Atoi(width); err == nil {
				parts = append(parts, "width="+width)
			}
		}
	}
	switch {
	case hasClass(n, "image-align-center") || insideCenter(n):
		parts = append(parts, "align=center")
	case hasClass(n, "image-align-end"):
		parts = append(parts, "align=end")
	}
	if len(parts) == 0 {
		return ""
	}
	return "#" + strings.Join(parts, "&")
}

/*
insideCenter reports whether a picture sits in Intercom's centering
wrapper, directly or inside a link.
*/
func insideCenter(n *html.Node) bool {
	p := n.Parent
	if p != nil && p.Type == html.ElementNode && p.Data == "a" {
		p = p.Parent
	}
	return p != nil && hasClass(p, intercomCenter)
}

/*
block writes Markdown as a block of its own.
*/
func block(w converter.Writer, markdown string) {
	_, _ = w.WriteString("\n\n" + markdown + "\n\n")
}

/*
escapeLabel escapes what would end a directive's label early.
*/
func escapeLabel(s string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(s)
}

/*
quoteAttr quotes a directive attribute value, percent-encoding a double
quote, which no address needs literally.
*/
func quoteAttr(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, "%22") + `"`
}

/*
collapse joins the words of text with single spaces.
*/
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func textOf(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textOf(c))
	}
	return b.String()
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func setAttr(n *html.Node, key, value string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: value})
}

func hasClass(n *html.Node, class string) bool {
	value, _ := attr(n, "class")
	for c := range strings.FieldsSeq(value) {
		if c == class {
			return true
		}
	}
	return false
}

func findChild(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
	}
	return nil
}

func findDescendant(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tag {
			return c
		}
		if found := findDescendant(c, tag); found != nil {
			return found
		}
	}
	return nil
}
