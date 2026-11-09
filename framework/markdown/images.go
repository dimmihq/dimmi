/*
|--------------------------------------------------------------------------
| Image Layout
|--------------------------------------------------------------------------
|
| A picture can say how wide it is and where it sits in the address's
| fragment, as in "![Plans](/plans.png#width=60&align=center)". The
| fragment never reaches the file's address; the picture gets classes
| instead, never inline styles, so a strict content security policy and
| the sanitizer both let them through and the page's stylesheet decides
| what they look like. A fragment that says anything else is a plain
| part of the address and stays.
|
*/

package markdown

import (
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

/*
Bounds of an image's width, in percent of the text column, and the step
widths are rounded to, so the stylesheet has one class for each.
*/
const (
	minImageWidth  = 20
	maxImageWidth  = 100
	imageWidthStep = 5
)

/*
imageTransformer moves the layout out of every picture's address and
into its classes.
*/
type imageTransformer struct{}

/*
Transform lays out every picture in the document.
*/
func (imageTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		img, ok := node.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		dest, classes := imageLayout(string(img.Destination))
		if classes != "" {
			img.Destination = []byte(dest)
			img.SetAttributeString("class", []byte(classes))
		}
		return ast.WalkContinue, nil
	})
}

/*
imageLayout splits a layout fragment off an image address, returning
the bare address and the classes it asks for, in a stable order so the
same picture always renders the same HTML. Any other address comes back
unchanged with no classes.
*/
func imageLayout(dest string) (string, string) {
	at := strings.LastIndexByte(dest, '#')
	if at < 0 {
		return dest, ""
	}
	values, err := url.ParseQuery(dest[at+1:])
	if err != nil || len(values) == 0 {
		return dest, ""
	}
	var classes []string
	for key := range values {
		class, ok := layoutClass(key, values.Get(key))
		if !ok {
			return dest, ""
		}
		classes = append(classes, class)
	}
	slices.Sort(classes)
	return dest[:at], strings.Join(classes, " ")
}

/*
layoutClass is the class for one layout setting, if it is one.
*/
func layoutClass(key, value string) (string, bool) {
	switch key {
	case "width":
		n, err := strconv.Atoi(value)
		if err != nil {
			return "", false
		}
		n = min(max(n, minImageWidth), maxImageWidth)
		n = (n + imageWidthStep/2) / imageWidthStep * imageWidthStep
		return "image-width-" + strconv.Itoa(n), true
	case "align":
		if value == "start" || value == "center" || value == "end" {
			return "image-align-" + value, true
		}
	}
	return "", false
}
