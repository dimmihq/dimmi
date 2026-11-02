/*
|--------------------------------------------------------------------------
| Word Vocabulary
|--------------------------------------------------------------------------
|
| The word diff library compares characters. To compare whole words, each
| distinct token is stood in for by one character, the library diffs
| those, and the result is expanded back into the tokens. The stand-ins
| must be valid characters, so the surrogate range is skipped, which
| leaves room for a little over a million distinct tokens.
|
*/

package diff

import (
	"strings"
	"unicode"

	"github.com/sergi/go-diff/diffmatchpatch"
)

/*
The stand-in characters skip the surrogates, which no valid string can
hold, and stop at the last code point.
*/
const (
	surrogateStart rune = 0xD800
	surrogateSize  rune = 0x800
)

/*
vocabulary numbers the distinct tokens of the texts being compared. The
token a stand-in character means sits at tokens[tokenNumber(r)].
*/
type vocabulary struct {
	tokens []string
	index  map[string]rune
	next   rune
}

/*
newVocabulary returns an empty vocabulary.
*/
func newVocabulary() *vocabulary {
	return &vocabulary{index: map[string]rune{}}
}

/*
encode returns the text as one stand-in character per token. It reports
false when the texts hold more distinct tokens than there are stand-ins.
*/
func (v *vocabulary) encode(text string) ([]rune, bool) {
	parts := tokens(text)
	out := make([]rune, len(parts))

	for i, part := range parts {
		r, ok := v.index[part]
		if !ok {
			if v.next > unicode.MaxRune {
				return nil, false
			}

			r = v.next
			v.tokens = append(v.tokens, part)
			v.index[part] = r
			v.next = nextStandIn(r)
		}

		out[i] = r
	}

	return out, true
}

/*
decode expands the stand-in characters of each diff back into tokens.
*/
func (v *vocabulary) decode(diffs []diffmatchpatch.Diff) []diffmatchpatch.Diff {
	for i, d := range diffs {
		var text strings.Builder
		for _, r := range d.Text {
			text.WriteString(v.tokens[tokenNumber(r)])
		}

		diffs[i].Text = text.String()
	}

	return diffs
}

/*
nextStandIn returns the stand-in character that follows r.
*/
func nextStandIn(r rune) rune {
	if r+1 == surrogateStart {
		return surrogateStart + surrogateSize
	}

	return r + 1
}

/*
tokenNumber returns the position of the token a stand-in character
stands for.
*/
func tokenNumber(r rune) int {
	if r >= surrogateStart+surrogateSize {
		return int(r - surrogateSize)
	}

	return int(r)
}
