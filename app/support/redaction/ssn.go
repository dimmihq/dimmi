/*
|--------------------------------------------------------------------------
| Social Security Numbers
|--------------------------------------------------------------------------
|
| The United States number is written 123-45-6789. Numbers that were never
| issued, an area of 000, 666 or 900 and above, a group of 00 or a serial
| of 0000, are left alone. The rule is off by default because order and
| ticket numbers often share the shape.
|
*/

package redaction

import "regexp"

var ssnPattern = regexp.MustCompile(`\b(\d{3})-(\d{2})-(\d{4})\b`)

/*
ssnMask is what an issued number becomes.
*/
const ssnMask = "•••-••-••••"

/*
maskSSN returns the masked form of the match when the number could
have been issued.
*/
func maskSSN(match string) (string, bool) {
	parts := ssnPattern.FindStringSubmatch(match)
	if parts == nil {
		return match, false
	}

	area, group, serial := parts[1], parts[2], parts[3]
	if area == "000" || area == "666" || area >= "900" || group == "00" || serial == "0000" {
		return match, false
	}

	return ssnMask, true
}
