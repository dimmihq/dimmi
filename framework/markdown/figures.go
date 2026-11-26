/*
|--------------------------------------------------------------------------
| Figures
|--------------------------------------------------------------------------
|
| A picture alone in its paragraph with a title, as in
| "![Plans](/plans.png "The plans page")", is a figure: the title is its
| caption, shown under it. A picture can be a link the usual way,
| "[![Plans](/plans.png "Caption")](https://example.com)", and stays one
| inside the figure. A picture without a title is a plain picture.
|
*/

package markdown

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

/*
KindFigure is the kind of a captioned picture's node.
*/
var KindFigure = ast.NewNodeKind("Figure") //nolint:gochecknoglobals // a node kind is registered once, at init

/*
figureNode holds a picture, or a link around one, and its caption.
*/
type figureNode struct {
	ast.BaseBlock

	caption string
}

/*
Kind returns KindFigure.
*/
func (n *figureNode) Kind() ast.NodeKind {
	return KindFigure
}

/*
Dump writes the node for debugging.
*/
func (n *figureNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Caption": n.caption}, nil)
}

/*
figureTransformer turns captioned pictures into figures.
*/
type figureTransformer struct{}

/*
Transform replaces each paragraph that holds only a captioned picture
with a figure.
*/
func (figureTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	var paragraphs []*ast.Paragraph
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if p, ok := n.(*ast.Paragraph); ok && entering {
			paragraphs = append(paragraphs, p)
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	for _, p := range paragraphs {
		img, ok := soleImage(p)
		if !ok || strings.TrimSpace(string(img.Title)) == "" {
			continue
		}
		fig := &figureNode{caption: strings.TrimSpace(string(img.Title))}
		img.Title = nil
		for c := p.FirstChild(); c != nil; c = p.FirstChild() {
			fig.AppendChild(fig, c)
		}
		p.Parent().ReplaceChild(p.Parent(), p, fig)
	}
}

/*
soleImage returns the picture a paragraph holds and nothing else, on
its own or as the only content of a link.
*/
func soleImage(p *ast.Paragraph) (*ast.Image, bool) {
	if p.ChildCount() != 1 {
		return nil, false
	}
	child := p.FirstChild()
	if link, ok := child.(*ast.Link); ok {
		if link.ChildCount() != 1 {
			return nil, false
		}
		child = link.FirstChild()
	}
	img, ok := child.(*ast.Image)
	return img, ok
}
