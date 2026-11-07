/*
|--------------------------------------------------------------------------
| Folding Text For Comparison
|--------------------------------------------------------------------------
|
| People expect "Café", "café" and "CAFE" to match when they set up a
| rule or a keyword list, and they never expect two spellings of the
| same letter to differ just because one was pasted from elsewhere.
| Folding gives text one comparable form; it is for comparing, never for
| showing or storing.
|
*/

package str

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

/*
Fold returns the text composed the canonical way (NFC) and in lower
case, so equal-looking text compares equal regardless of case.
*/
func Fold(s string) string {
	return strings.ToLower(norm.NFC.String(s))
}

/*
FoldAccents is [Fold] that also drops accents and other combining marks,
so "Crème" compares equal to "creme".
*/
func FoldAccents(s string) string {
	stripped := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}

		return r
	}, norm.NFD.String(s))

	return strings.ToLower(norm.NFC.String(stripped))
}
