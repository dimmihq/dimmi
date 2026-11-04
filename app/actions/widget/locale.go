/*
|--------------------------------------------------------------------------
| Widget Locale
|--------------------------------------------------------------------------
|
| A widget's language setting is either auto, following the visitor's
| browser, or one language tag in canonical form, so the same language is
| never stored two ways.
|
*/

package widget

import "github.com/dimmihq/dimmi/framework/i18n"

/*
LocaleAuto means the widget follows the visitor's browser.
*/
const LocaleAuto = "auto"

/*
NormalizeWidgetLocale returns the locale setting in canonical form, and false
for text that is no language tag.
*/
func NormalizeWidgetLocale(raw string) (string, bool) {
	if raw == LocaleAuto {
		return raw, true
	}

	return i18n.Canonical(raw)
}
