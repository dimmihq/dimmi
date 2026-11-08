/*
|--------------------------------------------------------------------------
| Custom Attributes
|--------------------------------------------------------------------------
|
| Contacts and companies carry the workspace's own fields. Their keys are
| short snake_case names, a record holds at most a hundred of them, a text
| value holds at most 1,024 characters, and nesting is refused so every
| value can be shown and filtered as it is. A key set to null is removed.
|
*/

package attributes

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
Limits on custom attributes.
*/
const (
	maxAttributes     = 100
	maxAttributeChars = 1024
)

var attributeKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

/*
Merge writes the changes over the current custom attributes: a key set
to null is removed, any other is written. It returns the result and the
keys that changed, sorted.
*/
func Merge(current json.RawMessage, changes map[string]any) (json.RawMessage, []string, error) {
	attrs := map[string]any{}
	if len(current) > 0 {
		if err := json.Unmarshal(current, &attrs); err != nil {
			return nil, nil, fmt.Errorf("read custom attributes: %w", err)
		}
	}

	var changed []string

	for _, key := range slices.Sorted(maps.Keys(changes)) {
		value := changes[key]
		if err := check(key, value); err != nil {
			return nil, nil, err
		}

		old, had := attrs[key]
		if value == nil && had {
			delete(attrs, key)
			changed = append(changed, key)
		}

		if value != nil && (!had || fmt.Sprint(old) != fmt.Sprint(value)) {
			attrs[key] = value
			changed = append(changed, key)
		}
	}

	if len(attrs) > maxAttributes {
		return nil, nil, exceptions.ErrInvalidAttributes
	}

	out, err := json.Marshal(attrs)
	if err != nil {
		return nil, nil, fmt.Errorf("write custom attributes: %w", err)
	}

	return out, changed, nil
}

/*
check refuses a key that is no snake_case name, and a value that is not
text, a number, a boolean or null, or text too long.
*/
func check(key string, value any) error {
	if !attributeKey.MatchString(key) {
		return exceptions.ErrInvalidAttributes
	}

	switch v := value.(type) {
	case nil, bool, float64, json.Number:
		return nil
	case string:
		if len([]rune(v)) > maxAttributeChars {
			return exceptions.ErrInvalidAttributes
		}

		return nil
	}

	return exceptions.ErrInvalidAttributes
}

/*
ValidKey reports whether a key is a custom attribute name.
*/
func ValidKey(key string) bool {
	return attributeKey.MatchString(key)
}
