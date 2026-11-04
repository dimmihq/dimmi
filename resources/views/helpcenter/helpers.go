/*
|--------------------------------------------------------------------------
| Help Center Page Helpers
|--------------------------------------------------------------------------
|
| Small decisions the templates share: where a path points under the help
| center's base, how a date reads, and whether text on the brand color is
| black or white. Text color follows the brand's contrast, so a pale
| brand never carries white text nobody can read.
|
*/

package helpcenter

import (
	"math"
	"strconv"
	"strings"
	"time"
)

/*
Meta is what a page says about itself: its title, the line search
engines show, the key of the section it belongs to, the trail the top
bar shows to it, its Markdown twin's path when it has one, and whether
search engines should skip it.
*/
type Meta struct {
	Title       string
	Description string
	Section     string
	Trail       []Crumb
	Markdown    string
	NoIndex     bool
}

/*
Sections returns the portal's sections in the order its navigation
shows them: the help center first, then the sections the portal has.
*/
func Sections(c Center) []Section {
	return append([]Section{{Key: "help", Name: "Help center", Icon: "book", Path: "/"}}, c.Sections...)
}

/*
Day formats a date the way every page shows it, such as "October 28,
2026".
*/
func Day(t time.Time) string {
	return t.Format("January 2, 2006")
}

/*
ShortDay formats a date without its year, such as "Oct 28".
*/
func ShortDay(t time.Time) string {
	return t.Format("Jan 2")
}

/*
Stamp formats a date for a time element's datetime attribute.
*/
func Stamp(t time.Time) string {
	return t.Format(time.DateOnly)
}

/*
Initial returns the first letter of the help center's product, drawn on
the brand when there is no logo.
*/
func Initial(c Center) string {
	for _, r := range c.Product {
		return strings.ToUpper(string(r))
	}

	return "?"
}

/*
OnBrand returns "dark" when black text reads better than white on the
brand color, and "light" otherwise, by the WCAG contrast of each.
*/
func OnBrand(hex string) string {
	l, ok := luminance(hex)
	if !ok {
		return "light"
	}

	if (l+0.05)/0.05 > 1.05/(l+0.05) {
		return "dark"
	}

	return "light"
}

/*
Plural returns the count with the noun, made plural unless the count is
one.
*/
func Plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}

	return strconv.Itoa(n) + " " + noun + "s"
}

/*
luminance returns the relative luminance of a #RRGGBB color.
*/
func luminance(hex string) (float64, bool) {
	if len(hex) != 7 || hex[0] != '#' {
		return 0, false
	}

	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		return 0, false
	}

	channel := func(shift uint) float64 {
		c := float64((v>>shift)&0xff) / 255
		if c <= 0.03928 {
			return c / 12.92
		}

		return math.Pow((c+0.055)/1.055, 2.4)
	}

	return 0.2126*channel(16) + 0.7152*channel(8) + 0.0722*channel(0), true
}
