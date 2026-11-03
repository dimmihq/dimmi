/*
|--------------------------------------------------------------------------
| String Helpers
|--------------------------------------------------------------------------
|
| The handful of string operations every part of an application reaches
| for: changing case, making URL slugs, pluralizing labels, shortening
| and masking text for display, and making random tokens. They all work
| on characters rather than bytes, so text in any language is never cut
| in the middle of a character.
|
*/

package str

import (
	"crypto/rand"
	"strings"

	"github.com/gertd/go-pluralize"
	"github.com/gosimple/slug"
	"github.com/iancoleman/strcase"
)

const (
	base62   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ellipsis = "..."
)

/*
unbiasedBytes and unbiasedDigitBytes are the largest multiples of 62
and of 10 a byte can hold. Random bytes at or above them are dropped so
every character is equally likely.
*/
const (
	unbiasedBytes      = 248
	unbiasedDigitBytes = 250
)

/*
Random returns n characters drawn evenly from letters and digits using
crypto/rand, suitable for tokens and secrets. It returns "" when n is
not positive. The random reader never fails, so its result is ignored.
*/
func Random(n int) string {
	return draw(n, base62, unbiasedBytes)
}

/*
RandomDigits returns n decimal digits drawn evenly using crypto/rand,
for codes a person types, such as one mailed to confirm a sign-in. It
returns "" when n is not positive.
*/
func RandomDigits(n int) string {
	return draw(n, base62[:10], unbiasedDigitBytes)
}

/*
draw draws n characters evenly from the alphabet, dropping random
bytes at or above the unbiased bound, the largest multiple of the
alphabet's size a byte can hold.
*/
func draw(n int, alphabet string, unbiased byte) string {
	out := make([]byte, 0, max(n, 0))
	buf := make([]byte, max(n, 1))

	for len(out) < n {
		_, _ = rand.Read(buf)

		for _, b := range buf {
			if b < unbiased && len(out) < n {
				out = append(out, alphabet[int(b)%len(alphabet)])
			}
		}
	}

	return string(out)
}

/*
Slug returns a lowercase URL slug: accents transliterated, every run of
other characters turned into one hyphen, as in "hello-world".
*/
func Slug(s string) string {
	return slug.Make(s)
}

/*
Snake returns the text in snake_case, as in "user_name".
*/
func Snake(s string) string {
	return strcase.ToSnake(s)
}

/*
Kebab returns the text in kebab-case, as in "user-name".
*/
func Kebab(s string) string {
	return strcase.ToKebab(s)
}

/*
Studly returns the text in StudlyCase, as in "UserName".
*/
func Studly(s string) string {
	return strcase.ToCamel(s)
}

/*
Plural returns the plural of an English word, keeping its case, as in
"person" to "people". Each call builds the rule set afresh, costing
about a tenth of a millisecond, so it suits labels and messages rather
than tight loops.
*/
func Plural(word string) string {
	return pluralize.NewClient().Plural(word)
}

/*
Singular returns the singular of an English word, keeping its case, as
in "people" to "person". It costs the same as Plural.
*/
func Singular(word string) string {
	return pluralize.NewClient().Singular(word)
}

/*
Limit shortens the text to at most n characters and appends "..." when
anything was cut, dropping trailing whitespace before the ellipsis.
*/
func Limit(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}

	return strings.TrimRight(string(runes[:max(n, 0)]), " \t\r\n") + ellipsis
}

/*
Mask replaces characters with char, starting at index and covering
length characters. A negative index counts from the end. A positive
length covers that many characters, a negative one stops that many
characters before the end, and zero covers the rest of the text. So
Mask("4242424242424242", '*', 0, -4) hides all but the last four
digits.
*/
func Mask(s string, char rune, index, length int) string {
	runes := []rune(s)
	start, end := maskSpan(len(runes), index, length)

	for i := start; i < end; i++ {
		runes[i] = char
	}

	return string(runes)
}

func maskSpan(size, index, length int) (start, end int) {
	start = min(index, size)
	if index < 0 {
		start = max(size+index, 0)
	}

	end = size
	if length > 0 {
		end = min(start+length, size)
	}

	if length < 0 {
		end = max(size+length, start)
	}

	return start, end
}
