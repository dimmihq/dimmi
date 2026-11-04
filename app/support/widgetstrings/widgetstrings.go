/*
|--------------------------------------------------------------------------
| Widget Strings
|--------------------------------------------------------------------------
|
| The messenger's built-in words, embedded in the binary. They are apart
| from the dashboard's language files because the two lists of languages are
| different: a visitor can read a messenger in languages the dashboard does
| not speak.
|
| A customer's own wording in their language beats a generic string, even
| for a visitor reading another language: a greeting the customer wrote means
| something a translated default does not. That is the one rule that makes
| the order below unusual.
|
*/

package widgetstrings

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/i18n"

	widgetlang "github.com/dimmihq/dimmi/resources/widget/lang"
)

/*
Fallback is the language every built-in word exists in.
*/
const Fallback = "en"

/*
Strings holds the built-in words by language.
*/
type Strings struct {
	byLocale map[string]map[string]string
	locales  []string
}

/*
Load reads every language file.
*/
func Load() (*Strings, error) {
	return From(widgetlang.Files())
}

/*
From reads every language file of the folder.
*/
func From(files fs.FS) (*Strings, error) {
	entries, err := fs.Glob(files, "*.json")
	if err != nil {
		return nil, fmt.Errorf("list widget languages: %w", err)
	}

	out := &Strings{byLocale: map[string]map[string]string{}}

	for _, name := range entries {
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, fmt.Errorf("read widget language %s: %w", name, err)
		}

		words := map[string]string{}
		if err := json.Unmarshal(data, &words); err != nil {
			return nil, fmt.Errorf("read widget language %s: %w", name, err)
		}

		out.byLocale[strings.TrimSuffix(name, ".json")] = words
	}

	out.locales = slices.Sorted(maps.Keys(out.byLocale))
	out.locales = slices.DeleteFunc(out.locales, func(l string) bool { return l == Fallback })
	out.locales = slices.Insert(out.locales, 0, Fallback)

	return out, nil
}

/*
Locales returns the languages shipped, the fallback first.
*/
func (s *Strings) Locales() []string {
	return slices.Clone(s.locales)
}

/*
Request is what decides a visitor's language: what the browser asked for, a
language the widget is fixed to, the language its texts are written in, and
the languages it has its own translations for.
*/
type Request struct {
	Requested    string
	Forced       string
	Default      string
	Translations []string
}

/*
Negotiate returns the language a visitor reads, always one of a bounded set,
so what a visitor sends can never grow a cache.
*/
func (s *Strings) Negotiate(r Request) string {
	wanted := r.Forced
	if wanted == "" {
		wanted = r.Requested
	}

	tag, ok := i18n.Canonical(wanted)
	if !ok {
		return r.fallback()
	}

	if exact := s.exact(tag, r); exact != "" {
		return exact
	}

	if base := Base(tag); base != tag {
		if exact := s.exact(base, r); exact != "" {
			return exact
		}
	}

	return r.fallback()
}

/*
exact returns the available language equal to the tag.
*/
func (s *Strings) exact(tag string, r Request) string {
	for _, list := range [][]string{r.Translations, {r.Default}, s.locales} {
		if i := slices.IndexFunc(list, func(l string) bool { return strings.EqualFold(l, tag) }); i >= 0 {
			return list[i]
		}
	}

	return ""
}

/*
fallback returns the language used when nothing the visitor asked for
exists.
*/
func (r Request) fallback() string {
	if r.Default != "" {
		return r.Default
	}

	return Fallback
}

/*
Direction returns which way the language reads.
*/
func Direction(locale string) string {
	base, _, _ := strings.Cut(locale, "-")
	if slices.Contains([]string{"ar", "he", "fa", "ur"}, base) {
		return "rtl"
	}

	return "ltr"
}

/*
Base returns the language without its region.
*/
func Base(locale string) string {
	base, _, _ := strings.Cut(locale, "-")

	return base
}

/*
Word returns one built-in word in the language, falling back to its base
language and then to English.
*/
func (s *Strings) Word(locale, key string) string {
	for _, candidate := range []string{locale, Base(locale), Fallback} {
		if word, ok := s.byLocale[candidate][key]; ok {
			return word
		}
	}

	return ""
}

/*
Resolve returns every word of a visitor's language given the customer's own
texts, written in their default language: the visitor's language where the
customer wrote nothing, the customer's words where they did.
*/
func (s *Strings) Resolve(locale, defaultLocale string, stored []byte) (map[string]string, error) {
	return s.ResolveWith(locale, defaultLocale, stored, nil)
}

/*
ResolveWith is Resolve with the customer's own translations, by language.
Each key goes: translation of the language or its base; then, when the
customer wrote nothing for it, the built-in words of the language or its
base; then the customer's words, the built-in words of their default
language, and English.
*/
func (s *Strings) ResolveWith(
	locale, defaultLocale string, stored []byte, translations map[string]map[string]string,
) (map[string]string, error) {
	texts := map[string]string{}
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &texts); err != nil {
			return nil, fmt.Errorf("read widget texts: %w", err)
		}
	}

	out := make(map[string]string, len(s.byLocale[Fallback]))

	for key := range s.byLocale[Fallback] {
		out[key] = s.resolveKey(key, locale, defaultLocale, written{texts: texts, translations: translations})
	}

	return out, nil
}

/*
written is what the customer wrote: texts in their default language and
translations by language.
*/
type written struct {
	texts        map[string]string
	translations map[string]map[string]string
}

/*
resolveKey resolves one key.
*/
func (s *Strings) resolveKey(key, locale, defaultLocale string, w written) string {
	for _, candidate := range []string{locale, Base(locale)} {
		if word := w.translations[candidate][key]; word != "" {
			return word
		}
	}

	if w.texts[key] != "" {
		return w.texts[key]
	}

	for _, candidate := range []string{locale, Base(locale), defaultLocale, Fallback} {
		if word, ok := s.byLocale[candidate][key]; ok && word != "" {
			return word
		}
	}

	return ""
}
