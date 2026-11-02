/*
|--------------------------------------------------------------------------
| Plural Forms
|--------------------------------------------------------------------------
|
| Languages count differently: English has "1 item" and "2 items", while
| French says "0 élément" and Polish has four forms. The "count" argument
| picks the form by the CLDR rules of the message's locale, decimals
| included, so "1.5" may read differently from "1". A message without
| the picked form uses its "other" form.
|
*/

package i18n

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
)

/*
pick returns the text of the message for the count among the arguments,
or its plain text without one. False means the count is not a number.
*/
func pick(tag language.Tag, forms entry, args Args) (string, bool) {
	count, ok := args["count"]
	if !ok {
		text, ok := forms[categoryOther]

		return text, ok
	}

	form, ok := pluralForm(tag, count)
	if !ok {
		return "", false
	}

	if text, ok := forms[categories()[form]]; ok {
		return text, true
	}

	text, ok := forms[categoryOther]

	return text, ok
}

/*
pluralForm returns the plural form the locale uses for the count, which
is an integer, a float or a decimal written as text such as "1.50".
*/
func pluralForm(tag language.Tag, count any) (plural.Form, bool) {
	written := fmt.Sprint(count)

	switch number := count.(type) {
	case float64:
		written = strconv.FormatFloat(number, 'f', -1, 64)
	case float32:
		written = strconv.FormatFloat(float64(number), 'f', -1, 32)
	}

	integer, fraction, _ := strings.Cut(strings.TrimLeft(written, "+-"), ".")
	if !isDigits(integer) || fraction != "" && !isDigits(fraction) {
		return plural.Other, false
	}

	digits := []byte(integer + fraction)
	for i := range digits {
		digits[i] -= '0'
	}

	return plural.Cardinal.MatchDigits(tag, digits, len(integer), len(fraction)), true
}

/*
isDigits reports whether the text is one or more decimal digits.
*/
func isDigits(text string) bool {
	return text != "" && strings.Trim(text, "0123456789") == ""
}
