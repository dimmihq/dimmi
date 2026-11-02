/*
|--------------------------------------------------------------------------
| Links, Images and Warnings
|--------------------------------------------------------------------------
|
| Before rendering, every link and image is shown to the caller's rewrite
| hook, which knows things the renderer cannot: which internal pages
| exist, where assets live and how large they are, which hosts are the
| site's own. The hook can point a link somewhere else, drop it and keep
| its text, or turn an image into a plain link.
|
| The same pass collects the problems an editor should fix before
| publishing: images without alt text, skipped heading levels, an empty
| document, and whatever the hook flags.
|
*/

package markdown

import (
	"net/url"
	"strconv"

	"github.com/yuin/goldmark/ast"
)

/*
Warning codes the renderer finds on its own. The rewrite hook may add
codes of its own through Link.Warning.
*/
const (
	WarnMissingAlt  = "missing_alt"
	WarnHeadingSkip = "heading_skip"
	WarnEmptyBody   = "empty_body"
)

/*
externalRel is the rel a link to another host gets unless the rewrite
hook says otherwise: no ranking credit, no window access, and a mark
that people, not the site, wrote it.
*/
const externalRel = "nofollow noopener ugc"

/*
Link is a link or image as the rewrite hook sees it and returns it.

URL and Title come from the Markdown. Rel starts as "nofollow noopener
ugc" for URLs with a host and empty otherwise; the default policy adds
nofollow to any URL with a host whatever Rel says, so rewrite links to
the site's own pages as paths to keep them followed.

Image is true for images; returning false renders the image as a link
to its URL with the alt text, or the URL when there is none. Width and
Height, when positive, are written on images.

Returning an empty URL drops the link and keeps its text, or keeps only
the alt text of an image. A non-empty Warning is added to the result's
warnings with the original URL as its detail.
*/
type Link struct {
	URL     string
	Title   string
	Rel     string
	Image   bool
	Width   int
	Height  int
	Warning string
}

/*
Warning is a problem found while rendering. Code names the kind of
problem and Detail points at where it is, such as the URL or heading
text.
*/
type Warning struct {
	Code   string
	Detail string
}

/*
inspector rewrites links and collects warnings over one document.
*/
type inspector struct {
	src      []byte
	rewrite  func(Link) Link
	warnings []Warning
	level    int
}

/*
inspect rewrites the links and images of a parsed document in place and
returns the warnings found.
*/
func inspect(doc ast.Node, src []byte, rewrite func(Link) Link) []Warning {
	ins := &inspector{src: src, rewrite: rewrite}

	if doc.ChildCount() == 0 {
		ins.warn(WarnEmptyBody, "")
	}

	for _, n := range collect(doc, nil) {
		switch n := n.(type) {
		case *ast.AutoLink:
			ins.link(replaceAutoLink(n, src))
		case *ast.Link:
			ins.link(n)
		case *ast.Image:
			ins.image(n)
		case *ast.Heading:
			ins.heading(n)
		}
	}

	return ins.warnings
}

/*
collect returns the links, images and headings under n in document
order, so the tree can change while they are visited.
*/
func collect(n ast.Node, out []ast.Node) []ast.Node {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch c.Kind() {
		case ast.KindAutoLink, ast.KindLink, ast.KindImage, ast.KindHeading:
			out = append(out, c)
		}

		out = collect(c, out)
	}

	return out
}

/*
link applies the rewrite hook to a link.
*/
func (ins *inspector) link(n *ast.Link) {
	out := ins.apply(Link{URL: string(n.Destination), Title: string(n.Title)})
	if out.URL == "" {
		unwrap(n)

		return
	}

	setLink(n, out)
}

/*
image applies the rewrite hook to an image and checks its alt text.
*/
func (ins *inspector) image(n *ast.Image) {
	if inlineText(n, ins.src) == "" {
		ins.warn(WarnMissingAlt, string(n.Destination))
	}

	out := ins.apply(Link{URL: string(n.Destination), Title: string(n.Title), Image: true})

	switch {
	case out.URL == "":
		unwrap(n)
	case !out.Image:
		link := ast.NewLink()
		moveChildren(n, link)

		if link.ChildCount() == 0 {
			link.AppendChild(link, ast.NewString([]byte(out.URL)))
		}

		n.Parent().ReplaceChild(n.Parent(), n, link)
		setLink(link, out)
	default:
		setImage(n, out)
	}
}

/*
heading warns when a heading is more than one level below the one
before it, which breaks the outline screen readers navigate by.
*/
func (ins *inspector) heading(n *ast.Heading) {
	if ins.level > 0 && n.Level > ins.level+1 {
		ins.warn(WarnHeadingSkip, inlineText(n, ins.src))
	}

	ins.level = n.Level
}

/*
apply fills in the default rel and runs the rewrite hook.
*/
func (ins *inspector) apply(link Link) Link {
	if u, err := url.Parse(link.URL); err == nil && u.Host != "" {
		link.Rel = externalRel
	}

	original := link.URL

	if ins.rewrite != nil {
		link = ins.rewrite(link)
	}

	if link.Warning != "" {
		ins.warn(link.Warning, original)
	}

	return link
}

/*
warn records a warning.
*/
func (ins *inspector) warn(code, detail string) {
	ins.warnings = append(ins.warnings, Warning{Code: code, Detail: detail})
}

/*
setLink writes a rewritten link onto its node.
*/
func setLink(n *ast.Link, link Link) {
	n.Destination = []byte(link.URL)
	n.Title = optional(link.Title)

	if link.Rel != "" {
		n.SetAttributeString("rel", []byte(link.Rel))
	}
}

/*
setImage writes a rewritten image onto its node, asking the browser to
load it lazily without blocking the page.
*/
func setImage(n *ast.Image, link Link) {
	n.Destination = []byte(link.URL)
	n.Title = optional(link.Title)
	n.SetAttributeString("loading", []byte("lazy"))
	n.SetAttributeString("decoding", []byte("async"))

	if link.Width > 0 {
		n.SetAttributeString("width", []byte(strconv.Itoa(link.Width)))
	}

	if link.Height > 0 {
		n.SetAttributeString("height", []byte(strconv.Itoa(link.Height)))
	}
}

/*
optional returns nil for an empty string, so an empty title is left
out of the HTML rather than written as title="".
*/
func optional(s string) []byte {
	if s == "" {
		return nil
	}

	return []byte(s)
}

/*
replaceAutoLink swaps a bare URL or email address for an ordinary link
with the same text, so the rewrite hook sees one kind of link.
*/
func replaceAutoLink(n *ast.AutoLink, src []byte) *ast.Link {
	link := ast.NewLink()
	link.Destination = n.URL(src)

	if n.AutoLinkType == ast.AutoLinkEmail {
		link.Destination = append([]byte("mailto:"), link.Destination...)
	}

	link.AppendChild(link, ast.NewString(n.Label(src)))
	n.Parent().ReplaceChild(n.Parent(), n, link)

	return link
}

/*
unwrap replaces a node with its children.
*/
func unwrap(n ast.Node) {
	parent := n.Parent()
	for c := n.FirstChild(); c != nil; c = n.FirstChild() {
		parent.InsertBefore(parent, n, c)
	}

	parent.RemoveChild(parent, n)
}

/*
moveChildren moves every child of from to the end of to.
*/
func moveChildren(from, to ast.Node) {
	for c := from.FirstChild(); c != nil; c = from.FirstChild() {
		to.AppendChild(to, c)
	}
}
