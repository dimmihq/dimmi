/*
|--------------------------------------------------------------------------
| Cleaning Typed Text
|--------------------------------------------------------------------------
|
| Short text people type, such as a name or a label, often carries
| characters nobody sees: control characters pasted from elsewhere,
| zero-width spaces, or the same letter encoded two ways. Left in, they
| make two equal-looking names differ and let one impersonate another.
| Cleaning composes every character the canonical way and drops the
| invisible ones, keeping the zero-width joiner that holds emoji
| sequences such as a family together.
|
*/

package str

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

/*
zeroWidthJoiner glues emoji into one sequence and is kept.
*/
const zeroWidthJoiner = '‍'

/*
Clean returns the text in Unicode composed form (NFC), without control
characters or invisible format characters other than the zero-width
joiner, and with surrounding white space trimmed.
*/
func Clean(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == zeroWidthJoiner {
			return r
		}

		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}

		return r
	}, norm.NFC.String(s))

	return strings.TrimSpace(cleaned)
}
