/*
|--------------------------------------------------------------------------
| Sentences
|--------------------------------------------------------------------------
|
| Where a sentence ends depends on the script: a full stop in English, a
| different mark in Arabic or Japanese, and a full stop that ends
| nothing in "3.5" or "e.g.". The Unicode sentence-boundary rules
| (UAX #29) settle it for every script the same way, so code that lays
| text out never has to know a language's punctuation.
|
*/

package str

import (
	"strings"

	"github.com/rivo/uniseg"
)

/*
Sentences cuts text into its sentences by the Unicode sentence-boundary
rules. Each keeps the space that follows it, so joining them gives the
text back unchanged.
*/
func Sentences(s string) []string {
	var out []string
	state := -1
	for rest := s; rest != ""; {
		var sentence string
		sentence, rest, state = uniseg.FirstSentenceInString(rest, state)
		if strings.TrimSpace(sentence) == "" && len(out) > 0 {
			out[len(out)-1] += sentence
			continue
		}
		out = append(out, sentence)
	}

	return out
}
