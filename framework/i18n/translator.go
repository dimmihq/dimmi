/*
|--------------------------------------------------------------------------
| Translator
|--------------------------------------------------------------------------
|
| Every message a person reads is looked up by key in the locale of the
| request, so the same code serves every language. Messages live in JSON
| files grouped by locale, one file per area of the application, and a
| key names its file first: "auth.failed" is the "failed" message of
| lang/en/auth.json.
|
|   lang/en/auth.json  {"failed": "These credentials do not match."}
|   lang/en/items.json {"count": {"one": "{{.count}} item",
|                                 "other": "{{.count}} items"}}
|
| The framework ships its own English messages, and the application's
| files are loaded after them, so the application can reword any of them
| and add every other language without touching code.
|
| A message missing from the request's locale falls back to the default
| locale, then to English, the language every message is written in
| first. A key missing everywhere comes back as the key itself, so a
| forgotten translation shows up on screen instead of failing a request.
|
*/

package i18n

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"golang.org/x/text/language"
)

/*
Source is the locale every message is first written in and the last
fallback for a message no other locale has.
*/
const Source = "en"

/*
Config configures translations. Locale is the default locale, used when
a request names none and for messages its own locale lacks. Locales
lists the locales requests may be served in; the default locale is
always one of them, and an empty list means every locale with files.
*/
type Config struct {
	Locale  string   `default:"en" koanf:"locale"`
	Locales []string `koanf:"locales"`
}

/*
Args are the values a message refers to by name, as {{.name}}. The
"count" value also picks the plural form of the message.
*/
type Args map[string]any

/*
Translator looks up messages by key and locale. It is safe for
concurrent use.
*/
type Translator struct {
	messages map[language.Tag]map[string]entry
	loaded   []language.Tag
	lookup   language.Matcher
	locales  []string
	matcher  language.Matcher
}

/*
New loads every <locale>/<group>.json file from each of the files in
order, typically the framework's messages followed by the application's
lang directory. A message loaded later replaces one with the same key.
*/
func New(cfg Config, files ...fs.FS) (*Translator, error) {
	fallback, err := language.Parse(cfg.Locale)
	if err != nil {
		return nil, fmt.Errorf("default locale %q: %w", cfg.Locale, err)
	}

	t := &Translator{messages: map[language.Tag]map[string]entry{fallback: {}}, loaded: []language.Tag{fallback}}

	for _, dir := range files {
		if err := t.load(dir); err != nil {
			return nil, err
		}
	}

	tags, err := supported(fallback, cfg.Locales, t.loaded)
	if err != nil {
		return nil, err
	}

	t.locales = make([]string, 0, len(tags))
	for _, tag := range tags {
		t.locales = append(t.locales, tag.String())
	}

	t.lookup = language.NewMatcher(t.loaded)
	t.matcher = language.NewMatcher(tags)

	return t, nil
}

/*
T returns the message for the key in the locale of the context, filled
with the arguments, or the key itself when no locale has the message.
*/
func (t *Translator) T(ctx context.Context, key string, args Args) string {
	message, ok := t.Lookup(ctx, key, args)
	if !ok {
		return key
	}

	return message
}

/*
Lookup returns the message for the key in the locale of the context and
whether any locale has it, for callers that keep their own text when a
message is missing.
*/
func (t *Translator) Lookup(ctx context.Context, key string, args Args) (string, bool) {
	return t.localize(Locale(ctx), key, args)
}

/*
Locales returns the locales requests may be served in, the default
locale first.
*/
func (t *Translator) Locales() []string {
	return slices.Clone(t.locales)
}

/*
Negotiate returns the served locale that best matches an Accept-Language
header, or the default locale when none does.
*/
func (t *Translator) Negotiate(acceptLanguage string) string {
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil {
		return t.locales[0]
	}

	_, index, _ := t.matcher.Match(tags...)

	return t.locales[index]
}

/*
Match returns the served locale closest to a preferred one, such as "fr"
for "fr-CA", and false when no served locale is close enough.
*/
func (t *Translator) Match(locale string) (string, bool) {
	tag, err := language.Parse(locale)
	if err != nil {
		return "", false
	}

	_, index, confidence := t.matcher.Match(tag)
	if confidence == language.No {
		return "", false
	}

	return t.locales[index], true
}

/*
localize renders the message in the locale, falling back to the default
locale and then to the source locale. False means no locale has a
message for the key, or the count picks a form the message lacks.
*/
func (t *Translator) localize(locale, key string, args Args) (string, bool) {
	for _, candidate := range []string{locale, Source} {
		tag, forms, ok := t.find(candidate, key)
		if !ok {
			continue
		}

		text, ok := pick(tag, forms, args)
		if ok {
			return render(text, args), true
		}
	}

	return "", false
}

/*
find returns the message for the key in the loaded locale closest to
the given one, or in the default locale when that one lacks it. A
locale that cannot be read is served as the default.
*/
func (t *Translator) find(locale, key string) (language.Tag, entry, bool) {
	tag := t.loaded[0]

	if parsed, err := language.Parse(locale); err == nil {
		_, index, _ := t.lookup.Match(parsed)
		tag = t.loaded[index]
	}

	for _, candidate := range []language.Tag{tag, t.loaded[0]} {
		if forms, ok := t.messages[candidate][key]; ok {
			return candidate, forms, true
		}
	}

	return language.Und, nil, false
}

/*
load adds the messages of every message file in the directory, each
locale's first file adding the locale to those loaded.
*/
func (t *Translator) load(files fs.FS) error {
	parsed, err := readAll(files)
	if err != nil {
		return err
	}

	for _, file := range parsed {
		if _, ok := t.messages[file.tag]; !ok {
			t.messages[file.tag] = map[string]entry{}
			t.loaded = append(t.loaded, file.tag)
		}

		maps.Copy(t.messages[file.tag], file.messages)
	}

	return nil
}

/*
supported returns the locales requests may be served in: the default
followed by the configured ones, or by every loaded locale when none
are configured.
*/
func supported(fallback language.Tag, configured []string, loaded []language.Tag) ([]language.Tag, error) {
	if len(configured) == 0 {
		return loaded, nil
	}

	tags := []language.Tag{fallback}

	for _, locale := range configured {
		tag, err := language.Parse(locale)
		if err != nil {
			return nil, fmt.Errorf("supported locale %q: %w", locale, err)
		}

		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}
