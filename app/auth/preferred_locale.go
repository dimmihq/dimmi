/*
|--------------------------------------------------------------------------
| Preferred Locale
|--------------------------------------------------------------------------
|
| A person reads dimmi in the language they picked, else in their
| workspace's, else in whatever their browser asks for. A choice the
| server no longer serves is skipped rather than refused, so removing a
| language from the install never locks anyone out of reading the app.
|
*/

package auth

import "github.com/dimmihq/dimmi/framework/i18n"

/*
PreferredLocale returns the first of the choices, in order, that the
translator serves, as the served locale it matches, and false when none
is set or served.
*/
func PreferredLocale(t *i18n.Translator, choices ...*string) (string, bool) {
	for _, choice := range choices {
		if choice == nil || *choice == "" {
			continue
		}

		if locale, ok := t.Match(*choice); ok {
			return locale, true
		}
	}

	return "", false
}
