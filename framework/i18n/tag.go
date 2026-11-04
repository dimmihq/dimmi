/*
|--------------------------------------------------------------------------
| Language Tags
|--------------------------------------------------------------------------
|
| Settings that name a language store it in one canonical spelling, so two
| ways of writing the same language never count as two languages, and text
| that is no language tag at all is refused before it is stored.
|
*/

package i18n

import "golang.org/x/text/language"

/*
Canonical returns the language tag in its canonical spelling, such as
"pt-BR" for "PT-br", and false for text that is no language tag.
*/
func Canonical(tag string) (string, bool) {
	parsed, err := language.Parse(tag)
	if err != nil {
		return "", false
	}

	return parsed.String(), true
}
