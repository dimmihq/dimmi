/*
|--------------------------------------------------------------------------
| Widget Texts By Language
|--------------------------------------------------------------------------
|
| A customer's own words for one language: a greeting for French visitors
| beside the one every other language shows. They are kept by language
| tag, each with the same keys and limits as the shared texts. A visitor
| reads their language's words first, then the shared ones, then the
| built-in words of their language.
|
*/

package widget

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/models"
)

/*
maxTextLocales is the most languages a messenger keeps words of its own
for.
*/
const maxTextLocales = 40

/*
mergeTextsByLocale returns the stored words by language with the change
applied. Each language's words merge as the shared texts do; null for a
language removes all of its words, and a language left with none is
dropped.
*/
func mergeTextsByLocale(stored json.RawMessage, change map[string]any) (json.RawMessage, error) {
	current := map[string]map[string]any{}
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &current); err != nil {
			return nil, fmt.Errorf("read widget texts by locale: %w", err)
		}
	}

	for raw, value := range change {
		if err := applyLocaleTexts(current, raw, value); err != nil {
			return nil, err
		}
	}

	if len(current) > maxTextLocales {
		return nil, failField("texts_by_locale", "Write texts for up to "+strconv.Itoa(maxTextLocales)+" languages.")
	}

	out, err := json.Marshal(current)
	if err != nil {
		return nil, fmt.Errorf("encode widget texts by locale: %w", err)
	}

	return out, nil
}

/*
applyLocaleTexts merges one language's change into the words by language.
*/
func applyLocaleTexts(current map[string]map[string]any, raw string, value any) error {
	locale, ok := i18n.Canonical(raw)
	if !ok || raw == LocaleAuto {
		return failField("texts_by_locale."+raw, "Use a language tag such as fr.")
	}

	if value == nil {
		delete(current, locale)

		return nil
	}

	words, isObject := value.(map[string]any)
	if !isObject {
		return failField("texts_by_locale."+raw, "Send the texts of the language as an object.")
	}

	cleaned, err := checkTextsAt("texts_by_locale."+raw, words)
	if err != nil {
		return err
	}

	current[locale] = mergeObject(current[locale], cleaned)
	if len(current[locale]) == 0 {
		delete(current, locale)
	}

	return nil
}

/*
TextsByLocale returns the widget's own words by language, none when it
has no words for any language.
*/
func TextsByLocale(w models.WidgetSetting) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	if len(w.TextsByLocale) == 0 {
		return out, nil
	}

	if err := json.Unmarshal(w.TextsByLocale, &out); err != nil {
		return nil, fmt.Errorf("read widget texts by locale: %w", err)
	}

	return out, nil
}
