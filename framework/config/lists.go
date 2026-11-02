/*
|--------------------------------------------------------------------------
| Lists From Single Values
|--------------------------------------------------------------------------
|
| An environment variable is always one string, so a list setting such as
| locales arrives as "en,fr". A list field given a single string takes it
| as a comma-separated list: items are trimmed and empty items dropped,
| so APP_LOCALES="en, fr," means [en fr]. Lists written as YAML lists are
| already lists and pass through untouched.
|
*/

package config

import (
	"reflect"
	"strings"

	"github.com/knadh/koanf/v2"
)

/*
splitList returns the trimmed, non-empty items of a comma-separated
list.
*/
func splitList(s string) []string {
	items := make([]string, 0, strings.Count(s, ",")+1)

	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}

	return items
}

/*
splitLists replaces every string value whose key belongs to a list field
of the target type with the list it spells.
*/
func splitLists(k *koanf.Koanf, target reflect.Type) error {
	for _, key := range listKeys(target, "") {
		s, ok := k.Get(key).(string)
		if !ok {
			continue
		}

		if err := k.Set(key, splitList(s)); err != nil {
			return err
		}
	}

	return nil
}

/*
listKeys returns the dotted configuration keys of the list fields in a
struct type, or in the struct a pointer type points at, following
nested structs. Pointer fields are not followed, so a type that refers
to itself cannot loop. Byte slices are not lists, and fields inside
maps are not known until the map is decoded.
*/
func listKeys(t reflect.Type, prefix string) []string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	var keys []string

	for field := range t.Fields() {
		key := strings.ToLower(fieldKey(field))
		if !field.IsExported() || key == "-" {
			continue
		}

		keys = append(keys, fieldListKeys(field.Type, prefix+key)...)
	}

	return keys
}

func fieldListKeys(t reflect.Type, key string) []string {
	if t.Kind() == reflect.Slice && t.Elem().Kind() != reflect.Uint8 {
		return []string{key}
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	return listKeys(t, key+".")
}
