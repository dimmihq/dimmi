/*
|--------------------------------------------------------------------------
| Heading IDs
|--------------------------------------------------------------------------
|
| Heading ids become URL fragments people share, so they are derived from
| the heading text the same way every time: lowercased, letters and
| digits of any script kept, spaces, hyphens and underscores turned into
| hyphens, everything else dropped. A heading in Japanese keeps its
| Japanese id instead of collapsing to a placeholder. Repeated ids get
| "-1", "-2" and so on, in document order.
|
*/

package markdown

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
)

/*
headingIDs hands out unique ids within one document.
*/
type headingIDs struct {
	used map[string]bool
	next map[string]int
}

/*
newHeadingIDs returns an empty id set for one document.
*/
func newHeadingIDs() *headingIDs {
	return &headingIDs{used: map[string]bool{}, next: map[string]int{}}
}

/*
Generate returns a unique id for the text of a heading or other element.
*/
func (s *headingIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	base := slug(string(value))
	if base == "" {
		base = "id"
		if kind == ast.KindHeading {
			base = "heading"
		}
	}

	id := base
	for n := s.next[base]; s.used[id]; n++ {
		id = base + "-" + strconv.Itoa(n+1)
		s.next[base] = n + 1
	}

	s.used[id] = true

	return []byte(id)
}

/*
Put marks an id as taken, for ids written explicitly in the document.
*/
func (s *headingIDs) Put(value []byte) {
	s.used[string(value)] = true
}

/*
slug turns heading text into the body of an id.
*/
func slug(text string) string {
	var out strings.Builder

	for _, r := range strings.TrimSpace(text) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			out.WriteRune(unicode.ToLower(r))
		case unicode.IsSpace(r), r == '-', r == '_':
			out.WriteByte('-')
		}
	}

	return out.String()
}
