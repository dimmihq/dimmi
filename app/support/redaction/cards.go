/*
|--------------------------------------------------------------------------
| Card Numbers
|--------------------------------------------------------------------------
|
| A run of 13 to 19 digits, written with or without spaces or dashes, is
| a card number only when its check digit is right and its length is
| one the card's network issues. Both checks together keep order numbers
| and phone numbers out, so the rule can be on by default. The masked
| form keeps the last four digits, which is what a teammate needs to
| tell one card from another.
|
*/

package redaction

import (
	"regexp"
	"slices"
	"strings"
)

var cardPattern = regexp.MustCompile(`\b\d(?:[ -]?\d){12,18}\b`)

/*
maskCard returns the masked form of the run when it is a card number.
*/
func maskCard(run string) (string, bool) {
	digits := onlyDigits(run)
	if len(digits) < 13 || len(digits) > 19 || !luhn(digits) || !issuedLength(digits) {
		return run, false
	}

	hidden := len(digits) - 4

	var b strings.Builder

	for i := range hidden {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}

		b.WriteString("•")
	}

	b.WriteByte(' ')
	b.WriteString(digits[hidden:])

	return b.String(), true
}

/*
onlyDigits drops the separators from the run.
*/
func onlyDigits(run string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}

		return -1
	}, run)
}

/*
luhn reports whether the digits carry a valid check digit.
*/
func luhn(digits string) bool {
	sum := 0
	double := false

	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}

		sum += d
		double = !double
	}

	return sum%10 == 0
}

/*
network is a range of leading digits a card network issues, and the
lengths its cards come in.
*/
type network struct {
	from, to string
	lengths  []int
}

/*
networks lists the issuer ranges, by their leading digits.
*/
func networks() []network {
	return []network{
		{"4", "4", []int{13, 16, 19}},
		{"51", "55", []int{16}},
		{"2221", "2720", []int{16}},
		{"34", "34", []int{15}},
		{"37", "37", []int{15}},
		{"6011", "6011", []int{16, 17, 18, 19}},
		{"644", "649", []int{16, 17, 18, 19}},
		{"65", "65", []int{16, 17, 18, 19}},
		{"300", "305", []int{14, 15, 16, 17, 18, 19}},
		{"36", "36", []int{14, 15, 16, 17, 18, 19}},
		{"38", "39", []int{16, 17, 18, 19}},
		{"3528", "3589", []int{16, 17, 18, 19}},
		{"62", "62", []int{16, 17, 18, 19}},
		{"50", "50", []int{13, 14, 15, 16, 17, 18, 19}},
		{"56", "58", []int{13, 14, 15, 16, 17, 18, 19}},
		{"67", "69", []int{13, 14, 15, 16, 17, 18, 19}},
	}
}

/*
issuedLength reports whether a network issues cards that start with
these digits and have this many of them.
*/
func issuedLength(digits string) bool {
	for _, n := range networks() {
		prefix := digits[:len(n.from)]
		if prefix < n.from || prefix > n.to {
			continue
		}

		if slices.Contains(n.lengths, len(digits)) {
			return true
		}
	}

	return false
}
