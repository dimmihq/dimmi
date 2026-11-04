/*
|--------------------------------------------------------------------------
| Contact Fields
|--------------------------------------------------------------------------
|
| What the inbox stores about a customer arrives from teammates, the API
| and channels, so it is normalized the same way everywhere: emails in
| lower case without surrounding space, phone numbers as digits with an
| optional leading plus. Custom attributes are the workspace's own
| fields: their keys are short snake_case names, a contact holds at most
| a hundred of them, and a text value holds at most 1,024 characters.
|
*/

package contacts

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"

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
NormalizeEmail returns the address in lower case without surrounding
space, nil when empty.
*/
func NormalizeEmail(email *string) *string {
	if email == nil {
		return nil
	}

	e := strings.ToLower(strings.TrimSpace(*email))
	if e == "" {
		return nil
	}

	return &e
}

/*
NormalizePhone returns the number as its digits, keeping a leading
plus, nil when it has none.
*/
func NormalizePhone(phone *string) *string {
	if phone == nil {
		return nil
	}

	p := strings.TrimSpace(*phone)
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}

		return -1
	}, p)

	if digits == "" {
		return nil
	}

	if strings.HasPrefix(p, "+") {
		digits = "+" + digits
	}

	return &digits
}

/*
MergeAttributes writes the changes over the current custom attributes:
a key set to null is removed, any other is written. It returns the
result and the keys that changed, sorted.
*/
func MergeAttributes(current json.RawMessage, changes map[string]any) (json.RawMessage, []string, error) {
	attrs := map[string]any{}
	if len(current) > 0 {
		if err := json.Unmarshal(current, &attrs); err != nil {
			return nil, nil, fmt.Errorf("read custom attributes: %w", err)
		}
	}

	var changed []string

	for _, key := range slices.Sorted(maps.Keys(changes)) {
		value := changes[key]
		if err := checkAttribute(key, value); err != nil {
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
checkAttribute refuses a key that is no snake_case name, and a value
that is not text, a number, a boolean or null, or text too long.
*/
func checkAttribute(key string, value any) error {
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
