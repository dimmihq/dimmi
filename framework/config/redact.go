/*
|--------------------------------------------------------------------------
| Redacted Configuration
|--------------------------------------------------------------------------
|
| Operators need to see the configuration the application actually
| loaded, most of all when boot fails, but printing it must never leak a
| credential into logs. Fields tagged secret:"true" are masked wherever
| they sit in the tree. Raw sections have no tags, because each driver
| decodes its own options, so their values are masked by key name
| instead. An empty secret stays empty, so a missing secret is still
| visible.
|
*/

package config

import (
	"fmt"
	"reflect"
	"strings"
)

const redactedValue = "[redacted]"

/*
Redacted returns the configuration as a map keyed like the YAML file,
with every non-empty field tagged secret:"true", and every non-empty
map value whose key names a secret, replaced by "[redacted]". Values
that are not structs or maps give an empty map.
*/
func Redacted(v any) map[string]any {
	out, ok := redact(reflect.ValueOf(v)).(map[string]any)
	if !ok {
		return map[string]any{}
	}

	return out
}

func redact(v reflect.Value) any {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}

		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		return redactStruct(v)
	}

	if v.Kind() == reflect.Map {
		return redactMap(v)
	}

	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		return redactList(v)
	}

	if !v.IsValid() {
		return nil
	}

	return v.Interface()
}

func redactStruct(v reflect.Value) map[string]any {
	out := map[string]any{}

	for i := range v.NumField() {
		field := v.Type().Field(i)

		name := fieldKey(field)
		if !field.IsExported() || name == "-" {
			continue
		}

		value := v.Field(i)
		if field.Tag.Get("secret") == "true" && !isEmpty(value) {
			out[name] = redactedValue

			continue
		}

		out[name] = redact(value)
	}

	return out
}

func redactMap(v reflect.Value) map[string]any {
	out := make(map[string]any, v.Len())

	for iter := v.MapRange(); iter.Next(); {
		key := fmt.Sprint(iter.Key().Interface())
		if isSecretKey(key) && !isEmpty(iter.Value()) {
			out[key] = redactedValue

			continue
		}

		out[key] = redact(iter.Value())
	}

	return out
}

func redactList(v reflect.Value) []any {
	out := make([]any, v.Len())

	for i := range v.Len() {
		out[i] = redact(v.Index(i))
	}

	return out
}

/*
fieldKey returns the key a field is loaded from: its koanf tag name, or
the field name when the tag is absent, as the decoder matches it.
*/
func fieldKey(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("koanf"), ",")
	if name == "" {
		return field.Name
	}

	return name
}

/*
isSecretKey reports whether a map key names a secret: it is, or ends
with an underscore and, one of secret, password, token, key, dsn,
credentials, private_key or signing_key. Matching whole words keeps
keyspace or monkey visible, and access_key_id, which only identifies a
key, ends in id and stays visible too.
*/
func isSecretKey(key string) bool {
	key = strings.ToLower(key)

	for _, word := range []string{
		"secret", "password", "token", "key", "dsn", "credentials", "private_key", "signing_key",
	} {
		if key == word || strings.HasSuffix(key, "_"+word) {
			return true
		}
	}

	return false
}

/*
isEmpty reports whether the value, followed through pointers and
interfaces, is unset, so an unset secret is shown rather than masked.
*/
func isEmpty(v reflect.Value) bool {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}

		v = v.Elem()
	}

	return !v.IsValid() || v.IsZero()
}
