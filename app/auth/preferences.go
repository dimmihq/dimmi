/*
|--------------------------------------------------------------------------
| Personal Preferences
|--------------------------------------------------------------------------
|
| Small choices about how the app behaves for one person, kept on their
| account and followed in every workspace. Each key is registered here
| with its default, so the server refuses a key it does not know and
| every reader sees a value for every key, stored or not. A person who
| never touched a preference keeps getting its default, even when that
| default changes later.
|
*/

package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
)

/*
The registered preference keys.
*/
const (
	PreferenceSendWithEnter = "send_with_enter"
)

/*
Preference failures.

ErrUnknownPreference refuses a key nobody registered.
ErrInvalidPreference refuses a value of the wrong type for its key.
*/
var (
	ErrUnknownPreference = errors.New("auth: unknown preference")
	ErrInvalidPreference = errors.New("auth: invalid preference value")
)

/*
PreferenceKeys returns every registered key, sorted.
*/
func PreferenceKeys() []string {
	return slices.Sorted(maps.Keys(preferenceDefaults()))
}

/*
preferenceDefaults is the value each registered key has until the person
changes it. A key's type is the type of its default.
*/
func preferenceDefaults() map[string]any {
	return map[string]any{
		PreferenceSendWithEnter: false,
	}
}

/*
ResolvePreferences returns every registered preference of the stored
document, with the default for each key the person never set. Stored
keys no longer registered, or holding a value of the wrong type, read as
their default or are left out.
*/
func ResolvePreferences(stored json.RawMessage) map[string]any {
	out := preferenceDefaults()

	var values map[string]json.RawMessage
	if len(stored) == 0 || json.Unmarshal(stored, &values) != nil {
		return out
	}

	for key, raw := range values {
		if value, err := decodePreference(key, raw); err == nil {
			out[key] = value
		}
	}

	return out
}

/*
MergePreferences applies a change to the stored document, one key at a
time: a value replaces the key's value, and null returns the key to its
default. Keys the change leaves out keep their value.

Errors: [ErrUnknownPreference] or [ErrInvalidPreference], each naming
the key, or a wrapped error for a stored document that does not read.
*/
func MergePreferences(stored json.RawMessage, change map[string]json.RawMessage) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &values); err != nil {
			return nil, fmt.Errorf("read stored preferences: %w", err)
		}
	}

	for _, key := range slices.Sorted(maps.Keys(change)) {
		raw := change[key]
		if err := CheckPreference(key, raw); err != nil {
			return nil, err
		}

		if isNull(raw) {
			delete(values, key)

			continue
		}

		values[key] = raw
	}

	return json.Marshal(values)
}

/*
CheckPreference reports whether the value is acceptable for the key:
registered, and null or of the default's type.

Errors: [ErrUnknownPreference] or [ErrInvalidPreference].
*/
func CheckPreference(key string, raw json.RawMessage) error {
	if _, ok := preferenceDefaults()[key]; !ok {
		return fmt.Errorf("%w: %s", ErrUnknownPreference, key)
	}

	if isNull(raw) {
		return nil
	}

	_, err := decodePreference(key, raw)

	return err
}

/*
decodePreference reads the value of a registered key, refusing one whose
type differs from the key's default.
*/
func decodePreference(key string, raw json.RawMessage) (any, error) {
	fallback, ok := preferenceDefaults()[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPreference, key)
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPreference, key)
	}

	if fmt.Sprintf("%T", value) != fmt.Sprintf("%T", fallback) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPreference, key)
	}

	return value, nil
}

/*
isNull reports whether the raw value is JSON null.
*/
func isNull(raw json.RawMessage) bool {
	return string(raw) == "null"
}
