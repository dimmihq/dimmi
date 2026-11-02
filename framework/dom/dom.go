/*
|--------------------------------------------------------------------------
| HTML Documents
|--------------------------------------------------------------------------
|
| A parsed HTML document that can be searched with CSS selectors and
| changed in place: drop quoted replies, read attributes, rewrite them,
| then render the result again. Parsing follows the HTML5 algorithm the
| browsers use, so broken markup from email clients and crawled pages
| becomes the same tree a browser would build.
|
| A document is not sanitized. Render it through a sanitize policy before
| it reaches a browser.
|
*/

package dom

import (
	"fmt"
	"io"

	"github.com/PuerkitoBio/goquery"
)

/*
Document is a parsed HTML document. It is not safe for concurrent use.
*/
type Document struct {
	doc *goquery.Document
}

/*
Selection is the set of elements a selector matched, in document order.
It may be empty, and every method works on an empty selection.
*/
type Selection struct {
	sel *goquery.Selection
}

/*
Parse reads a whole HTML document or fragment. Fragments are wrapped in
html, head and body elements as a browser would. The reader is read to
the end, so limit its size before passing untrusted input. Elements
nested more than 512 deep are refused rather than parsed.
*/
func Parse(r io.Reader) (*Document, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	return &Document{doc: doc}, nil
}

/*
Find returns the elements matching a CSS selector. A selector that does
not parse matches nothing.
*/
func (d *Document) Find(selector string) Selection {
	return Selection{sel: d.doc.Find(selector)}
}

/*
HTML renders the whole document, including the html, head and body
elements. Rendering only fails on a write error or on a tree no parser
builds, and it writes to memory, so the error is discarded.
*/
func (d *Document) HTML() string {
	out, _ := d.doc.Html()

	return out
}

/*
Find returns the descendants of the selected elements that match a CSS
selector.
*/
func (s Selection) Find(selector string) Selection {
	return Selection{sel: s.sel.Find(selector)}
}

/*
NextAll returns every following sibling element of the selected
elements.
*/
func (s Selection) NextAll() Selection {
	return Selection{sel: s.sel.NextAll()}
}

/*
Len returns how many elements are selected.
*/
func (s Selection) Len() int {
	return s.sel.Length()
}

/*
Each calls fn with each selected element on its own, in document order.
*/
func (s Selection) Each(fn func(Selection)) {
	s.sel.Each(func(_ int, el *goquery.Selection) {
		fn(Selection{sel: el})
	})
}

/*
Attr returns the value of an attribute of the first selected element,
and whether it is present.
*/
func (s Selection) Attr(name string) (string, bool) {
	return s.sel.Attr(name)
}

/*
SetAttr sets an attribute on every selected element.
*/
func (s Selection) SetAttr(name, value string) Selection {
	return Selection{sel: s.sel.SetAttr(name, value)}
}

/*
Text returns the combined text of the selected elements and their
descendants, with entities decoded.
*/
func (s Selection) Text() string {
	return s.sel.Text()
}

/*
HTML renders the contents of the first selected element. As with
Document.HTML, rendering into memory cannot fail.
*/
func (s Selection) HTML() string {
	out, _ := s.sel.Html()

	return out
}

/*
Remove takes the selected elements out of the document and returns
them.
*/
func (s Selection) Remove() Selection {
	return Selection{sel: s.sel.Remove()}
}
