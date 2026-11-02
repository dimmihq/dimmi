/*
|--------------------------------------------------------------------------
| Formatting
|--------------------------------------------------------------------------
|
| Numbers, money and dates are written the way a reader of the locale
| expects: "1,234.5" in English is "1 234,5" in French. Numbers and money
| follow the locale's own rules from golang.org/x/text. Dates follow the
| layouts each locale's "formats" messages give, so a translator decides
| how a date reads without code changing. Times stay UTC everywhere else
| and only take the reader's location when written for a person.
|
|   lang/fr/formats.json {"date": "02/01/2006", "time": "15:04"}
|
*/

package i18n

import (
	"cmp"
	"embed"
	"fmt"
	"io/fs"
	"math"
	"time"

	"golang.org/x/text/currency"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

//go:embed lang
var messages embed.FS

/*
Layouts used when no locale has a "formats" message, written as Go time
layouts.
*/
const (
	dateLayout = "January 2, 2006"
	timeLayout = "3:04 PM"
)

/*
Messages returns the framework's own English messages of this package,
the date and time layouts, as <locale>/<group>.json files.
*/
func Messages() fs.FS {
	sub, _ := fs.Sub(messages, "lang")

	return sub
}

/*
FormatNumber writes the number with the locale's separators, such as
"1,234.5" in English and "1.234,5" in German.
*/
func FormatNumber(locale string, value any) string {
	return printer(locale).Sprint(number.Decimal(value))
}

/*
FormatMoney writes an amount of money held in the currency's minor
unit, such as cents, with the currency's symbol and decimals and the
locale's separators: 123450 USD is "$ 1,234.50" in English. It fails
for a code that is not an ISO 4217 currency.
*/
func FormatMoney(locale string, minor int64, code string) (string, error) {
	unit, err := currency.ParseISO(code)
	if err != nil {
		return "", fmt.Errorf("currency %q: %w", code, err)
	}

	scale, _ := currency.Standard.Rounding(unit)
	amount := float64(minor) / math.Pow10(scale)

	return printer(locale).Sprint(currency.Symbol(unit.Amount(amount))), nil
}

/*
FormatDate writes the date of the moment as the locale writes dates, in
the location, or in UTC when the location is nil.
*/
func (t *Translator) FormatDate(locale string, at time.Time, loc *time.Location) string {
	return at.In(cmp.Or(loc, time.UTC)).Format(t.layout(locale, "formats.date", dateLayout))
}

/*
FormatTime writes the time of day of the moment as the locale writes
times, in the location, or in UTC when the location is nil.
*/
func (t *Translator) FormatTime(locale string, at time.Time, loc *time.Location) string {
	return at.In(cmp.Or(loc, time.UTC)).Format(t.layout(locale, "formats.time", timeLayout))
}

/*
layout returns the locale's layout for the key, or the fallback layout
when no locale has one.
*/
func (t *Translator) layout(locale, key, fallback string) string {
	layout, ok := t.localize(locale, key, nil)
	if !ok {
		return fallback
	}

	return layout
}

/*
printer formats for the locale, or for the source locale when the locale
cannot be read.
*/
func printer(locale string) *message.Printer {
	tag, err := language.Parse(locale)
	if err != nil {
		tag = language.Make(Source)
	}

	return message.NewPrinter(tag)
}
