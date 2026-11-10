/*
|--------------------------------------------------------------------------
| Fake Schema Checks
|--------------------------------------------------------------------------
|
| A real provider never returns an answer that breaks the schema it was
| given, so neither may the fake: a scripted reply that does is a bug
| in the test, and the call fails loudly instead of handing the code
| under test something no model would send. The checker covers the
| rules typed answers lean on — types, required keys, closed objects
| and enums — and nothing more.
|
*/

package fake

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
ErrSchemaMismatch marks a scripted reply that breaks the request's
response schema.
*/
var ErrSchemaMismatch = errors.New("llm fake: scripted reply breaks the response schema")

/*
checkAnswer validates a scripted reply against the request's schema.
*/
func checkAnswer(rs *contracts.ResponseSchema, text string) error {
	var schema map[string]any
	if err := json.Unmarshal(rs.Schema, &schema); err != nil {
		return fmt.Errorf("%w: schema %s is not a JSON object", ErrSchemaMismatch, rs.Name)
	}

	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return fmt.Errorf("%w: reply %q is not one JSON object", ErrSchemaMismatch, text)
	}

	if problem := mismatch(schema, value, "$"); problem != "" {
		return fmt.Errorf("%w: %s", ErrSchemaMismatch, problem)
	}
	return nil
}

/*
mismatch describes the first way a value breaks a schema node, or
returns empty when it fits.
*/
func mismatch(schema map[string]any, value any, path string) string {
	if alts, ok := schema["anyOf"].([]any); ok {
		return anyMismatch(alts, value, path)
	}
	if problem := typeMismatch(schema, value, path); problem != "" {
		return problem
	}
	if enum, ok := schema["enum"].([]any); ok && !slices.ContainsFunc(enum, same(value)) {
		return fmt.Sprintf("%s is not one of the allowed values", path)
	}

	switch v := value.(type) {
	case map[string]any:
		return objectMismatch(schema, v, path)
	case []any:
		return itemsMismatch(schema, v, path)
	}
	return ""
}

/*
itemsMismatch checks each item of an array against the items schema.
*/
func itemsMismatch(schema map[string]any, value []any, path string) string {
	items, _ := schema["items"].(map[string]any)
	for i, item := range value {
		if problem := mismatch(items, item, fmt.Sprintf("%s[%d]", path, i)); problem != "" {
			return problem
		}
	}
	return ""
}

/*
anyMismatch passes when the value fits at least one alternative.
*/
func anyMismatch(alts []any, value any, path string) string {
	for _, alt := range alts {
		if m, ok := alt.(map[string]any); ok && mismatch(m, value, path) == "" {
			return ""
		}
	}
	return path + " fits none of its alternatives"
}

/*
typeMismatch checks the node's type, a name or a list of names.
*/
func typeMismatch(schema map[string]any, value any, path string) string {
	var names []string
	switch t := schema["type"].(type) {
	case string:
		names = []string{t}
	case []any:
		for _, name := range t {
			if s, ok := name.(string); ok {
				names = append(names, s)
			}
		}
	}
	if len(names) == 0 || slices.Contains(names, typeOf(value)) {
		return ""
	}
	if typeOf(value) == "integer" && slices.Contains(names, "number") {
		return ""
	}
	return fmt.Sprintf("%s is a %s, not %v", path, typeOf(value), names)
}

/*
typeOf names a decoded JSON value's type; whole numbers are integers.
*/
func typeOf(value any) string {
	switch v := value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case float64:
		if v == float64(int64(v)) {
			return "integer"
		}
		return "number"
	}
	return "null"
}

/*
objectMismatch checks required keys, a closed object's extra keys and
each declared property.
*/
func objectMismatch(schema, value map[string]any, path string) string {
	props, _ := schema["properties"].(map[string]any)
	required, _ := schema["required"].([]any)
	for _, name := range required {
		key, _ := name.(string)
		if _, present := value[key]; !present {
			return fmt.Sprintf("%s is missing required %q", path, key)
		}
	}

	closed := schema["additionalProperties"] == false
	for key, item := range value {
		sub, declared := props[key].(map[string]any)
		if !declared && closed {
			return fmt.Sprintf("%s has %q, which the schema does not allow", path, key)
		}
		if problem := mismatch(sub, item, path+"."+key); problem != "" {
			return problem
		}
	}
	return ""
}

/*
same matches a decoded value against an enum entry by its JSON form.
*/
func same(value any) func(any) bool {
	want, _ := json.Marshal(value)
	return func(entry any) bool {
		got, _ := json.Marshal(entry)
		return bytes.Equal(got, want)
	}
}
