/*
|--------------------------------------------------------------------------
| Stripping Tags
|--------------------------------------------------------------------------
|
| A document too broken to parse still has words in it. Stripping its
| tags reads it one token at a time without building a tree, so no depth
| of nesting can stop it, while the caller can still have elements left
| out that a reader would never see.
|
*/

package dom

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
)

/*
Tag is an element's opening tag as StripTags meets it.
*/
type Tag struct {
	Name  string
	Attrs map[string]string
}

/*
stripper is the state of one pass over a document. Skipping names the
element being left out and depth counts how far inside it the pass is.
*/
type stripper struct {
	out      strings.Builder
	skip     func(Tag) bool
	skipping string
	depth    int
}

/*
StripTags returns the text of HTML, with entities decoded and block
elements on their own lines. It works on documents Parse refuses.
Scripts, styles and the head are left out, and so is every element the
skip function picks out, which may be nil.
*/
func StripTags(document string, skip func(Tag) bool) string {
	pass := stripper{skip: skip}
	tokens := html.NewTokenizer(strings.NewReader(document))

	for kind := tokens.Next(); kind != html.ErrorToken; kind = tokens.Next() {
		pass.take(tokens.Token())
	}

	return tidyLines(pass.out.String())
}

/*
take adds one token to the text, or counts it against the element being
left out.
*/
func (s *stripper) take(token html.Token) {
	if s.skipping != "" {
		s.count(token)

		return
	}

	switch token.Type {
	case html.TextToken:
		s.out.WriteString(token.Data)
	case html.StartTagToken:
		if s.dropped(token) {
			s.skipping, s.depth = token.Data, 1

			return
		}

		s.breakLine(token.Data)
	case html.EndTagToken, html.SelfClosingTagToken:
		s.breakLine(token.Data)
	case html.ErrorToken, html.CommentToken, html.DoctypeToken:
	}
}

/*
count follows the nesting of the element being left out, until it ends.
*/
func (s *stripper) count(token html.Token) {
	if token.Data != s.skipping {
		return
	}

	switch token.Type {
	case html.StartTagToken:
		s.depth++
	case html.EndTagToken:
		s.depth--
		if s.depth == 0 {
			s.skipping = ""
		}
	case html.ErrorToken, html.TextToken, html.SelfClosingTagToken, html.CommentToken, html.DoctypeToken:
	}
}

/*
dropped reports whether an element is left out with all it contains.
Elements that hold nothing, such as images, never start a skip.
*/
func (s *stripper) dropped(token html.Token) bool {
	switch token.Data {
	case "script", "style", "head", "title":
		return true
	case "br", "hr", "img", "input", "meta", "link", "wbr", "col", "area", "base", "embed", "source", "track":
		return false
	}

	if s.skip == nil {
		return false
	}

	attrs := make(map[string]string, len(token.Attr))
	for _, attr := range token.Attr {
		attrs[attr.Key] = attr.Val
	}

	return s.skip(Tag{Name: token.Data, Attrs: attrs})
}

/*
breakLine starts a new line at the edge of a block element.
*/
func (s *stripper) breakLine(name string) {
	blocks := []string{
		"address", "aside", "blockquote", "br", "center", "dd", "div", "dl", "dt", "footer", "h1", "h2",
		"h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "tr", "ul",
	}
	if slices.Contains(blocks, name) {
		s.out.WriteByte('\n')
	}
}

/*
tidyLines trims every line and collapses runs of blank lines.
*/
func tidyLines(text string) string {
	lines := strings.Split(text, "\n")
	kept := lines[:0]

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" && (len(kept) == 0 || kept[len(kept)-1] == "") {
			continue
		}

		kept = append(kept, line)
	}

	return strings.TrimSpace(strings.Join(kept, "\n"))
}
