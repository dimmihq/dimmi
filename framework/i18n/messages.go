/*
|--------------------------------------------------------------------------
| Message Files
|--------------------------------------------------------------------------
|
| A message file is a JSON object. A string is a message; an object whose
| keys are plural categories ("zero", "one", "two", "few", "many",
| "other") is one message with a form per category; any other object
| groups messages under its key, so {"throttle": {"seconds": "..."}} in
| auth.json is the message "auth.throttle.seconds". An object mixing
| plural categories with other keys is refused, since it cannot be read
| either way.
|
| Placeholders are written {{.name}} and filled with the argument of that
| name. They are plain substitutions with no logic: a message is text a
| translator writes, never code that runs.
|
*/

package i18n

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
)

/*
Errors a message file is refused with.

ErrMixedKeys means an object mixes plural categories with other keys.
ErrInvalidMessage means a value is neither a string nor an object.
*/
var (
	ErrMixedKeys      = errors.New("i18n: plural forms mixed with other keys")
	ErrInvalidMessage = errors.New("i18n: message is not text")
)

/*
entry is one message: its text per plural category, a plain message
having only "other".
*/
type entry map[string]string

/*
messageFile is one parsed message file: its path, its locale and its
messages, keyed by their full key, the file's group first.
*/
type messageFile struct {
	path     string
	tag      language.Tag
	messages map[string]entry
}

/*
readAll parses every <locale>/<group>.json file in the directory.
*/
func readAll(files fs.FS) ([]messageFile, error) {
	paths, err := fs.Glob(files, "*/*.json")
	if err != nil {
		return nil, fmt.Errorf("find message files: %w", err)
	}

	parsed := make([]messageFile, 0, len(paths))

	for _, file := range paths {
		messages, err := read(files, file)
		if err != nil {
			return nil, err
		}

		parsed = append(parsed, messages)
	}

	return parsed, nil
}

/*
read parses the messages of one file under its locale, each key
prefixed with the file's group name.
*/
func read(files fs.FS, file string) (messageFile, error) {
	dir, name := path.Split(file)

	tag, err := language.Parse(strings.TrimSuffix(dir, "/"))
	if err != nil {
		return messageFile{}, fmt.Errorf("locale of %s: %w", file, err)
	}

	contents, err := fs.ReadFile(files, file)
	if err != nil {
		return messageFile{}, fmt.Errorf("read %s: %w", file, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(contents, &raw); err != nil {
		return messageFile{}, fmt.Errorf("parse %s: %w", file, err)
	}

	messages := map[string]entry{}
	if err := flatten(strings.TrimSuffix(name, ".json"), raw, messages); err != nil {
		return messageFile{}, fmt.Errorf("parse %s: %w", file, err)
	}

	return messageFile{path: file, tag: tag, messages: messages}, nil
}

/*
flatten adds the messages of a group to out, each under the group's key
joined to its own by a dot.
*/
func flatten(prefix string, group map[string]any, out map[string]entry) error {
	for _, key := range slices.Sorted(maps.Keys(group)) {
		full := prefix + "." + key

		switch value := group[key].(type) {
		case string:
			out[full] = entry{categoryOther: value}
		case map[string]any:
			if err := nested(full, value, out); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: %w", full, ErrInvalidMessage)
		}
	}

	return nil
}

/*
nested adds an object found under a key: one plural message when its
keys are plural categories, otherwise a group of messages.
*/
func nested(key string, value map[string]any, out map[string]entry) error {
	forms := entry{}

	for category, text := range value {
		if s, ok := text.(string); ok && isCategory(category) {
			forms[category] = s
		}
	}

	switch len(forms) {
	case 0:
		return flatten(key, value, out)
	case len(value):
		out[key] = forms

		return nil
	default:
		return fmt.Errorf("%s: %w", key, ErrMixedKeys)
	}
}

/*
categoryOther is the plural category every message has: the text of a
message without plural forms.
*/
const categoryOther = "other"

/*
categories names the plural categories, indexed by plural form.
*/
func categories() [6]string {
	var names [6]string

	names[plural.Other] = categoryOther
	names[plural.Zero] = "zero"
	names[plural.One] = "one"
	names[plural.Two] = "two"
	names[plural.Few] = "few"
	names[plural.Many] = "many"

	return names
}

/*
isCategory reports whether the key names a plural category.
*/
func isCategory(key string) bool {
	names := categories()

	return slices.Contains(names[:], key)
}

/*
render fills each {{.name}} placeholder with the argument of that name.
A placeholder whose argument is missing, or that is not a plain name,
stays as written, so the mistake shows on screen.
*/
func render(text string, args Args) string {
	if !strings.Contains(text, "{{") {
		return text
	}

	var out strings.Builder

	for {
		start := strings.Index(text, "{{")
		if start < 0 {
			break
		}

		end := strings.Index(text[start:], "}}")
		if end < 0 {
			break
		}

		end += start + len("}}")
		out.WriteString(text[:start])
		out.WriteString(placeholder(text[start:end], args))
		text = text[end:]
	}

	out.WriteString(text)

	return out.String()
}

/*
placeholder returns the value a placeholder such as "{{.name}}" stands
for, or the placeholder itself when it has none.
*/
func placeholder(written string, args Args) string {
	inner := strings.TrimSpace(written[len("{{") : len(written)-len("}}")])

	name, ok := strings.CutPrefix(inner, ".")
	if !ok {
		return written
	}

	value, ok := args[name]
	if !ok {
		return written
	}

	return fmt.Sprint(value)
}
