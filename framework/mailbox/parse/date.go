/*
|--------------------------------------------------------------------------
| Dates
|--------------------------------------------------------------------------
|
| A date with a zone name says when it was written only if that name has
| one meaning. The standard defines a few, for the United States and for
| universal time. Anything else, such as CET, is a guess: read naively it
| would be taken as universal time, hours off and different on every
| server. Such a date counts as unreadable, so the time of arrival is
| used instead.
|
*/

package parse

import (
	"errors"
	"fmt"
	netmail "net/mail"
	"strings"
	"time"
	"unicode"
)

/*
errZoneOffset is the failure of a date whose zone name has no defined
offset.
*/
var errZoneOffset = errors.New("zone name without a defined offset")

/*
readDate parses a Date field, reading its zone name by the standard.
*/
func readDate(value string) (time.Time, error) {
	text, _, _ := strings.Cut(value, "(")
	text = strings.TrimSpace(text)

	zone := text[strings.LastIndexAny(text, " \t")+1:]
	if !isZoneName(zone) {
		return parseDate(text)
	}

	offset, known := zoneOffset(strings.ToUpper(zone))
	if !known {
		return time.Time{}, errZoneOffset
	}

	return parseDate(strings.TrimSuffix(text, zone) + offset)
}

func parseDate(text string) (time.Time, error) {
	when, err := netmail.ParseDate(text)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse date: %w", err)
	}

	return when, nil
}

func isZoneName(text string) bool {
	return text != "" && !strings.ContainsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) })
}

/*
zoneOffset returns the numeric offset of a zone name the standard
defines.
*/
func zoneOffset(zone string) (string, bool) {
	switch zone {
	case "UT", "UTC", "GMT", "Z":
		return "+0000", true
	case "EDT":
		return "-0400", true
	case "EST", "CDT":
		return "-0500", true
	case "CST", "MDT":
		return "-0600", true
	case "MST", "PDT":
		return "-0700", true
	case "PST":
		return "-0800", true
	}

	return "", false
}
