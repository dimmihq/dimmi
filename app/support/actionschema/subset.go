/*
|--------------------------------------------------------------------------
| Portable Schema Subset
|--------------------------------------------------------------------------
|
| An action's input schema is shown to whichever model the workspace
| runs, so it keeps to the part of JSON Schema every provider accepts:
| plain types, properties, required, enums and bounds, three levels
| deep at most and twenty properties in all. Combinators and references
| are refused outright. The root is always an object that allows no
| other properties, so the model can never slip in a field the admin did
| not declare.
|
*/

package actionschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
)

/*
Subset limits: nesting depth, properties across the whole schema, enum
values, pattern length and the length of a description.
*/
const (
	maxDepth       = 3
	maxProperties  = 20
	maxEnum        = 50
	maxPattern     = 200
	maxDescription = 300
	maxItems       = 50
	maxSchemaBytes = 16 << 10
)

/*
ErrUnsupported marks a schema outside the portable subset.
*/
var ErrUnsupported = errors.New("schema outside the supported subset")

/*
allowedKeywords are the keywords the subset keeps.
*/
func allowedKeywords() []string {
	return []string{
		"type", "properties", "required", "enum", "const", "description", "title", "minimum", "maximum",
		"exclusiveMinimum", "exclusiveMaximum", "minLength", "maxLength", "pattern", "format", "items",
		"minItems", "maxItems", "default", "examples", "additionalProperties",
	}
}

/*
allowedTypes are the types a property may have.
*/
func allowedTypes() []string {
	return []string{"string", "integer", "number", "boolean", "array", "object"}
}

/*
allowedFormats are the string formats the subset keeps.
*/
func allowedFormats() []string {
	return []string{"date", "date-time", "email", "uri"}
}

/*
Normalize checks a schema against the subset and returns it with a root
object that allows no other properties.
*/
func Normalize(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) > maxSchemaBytes {
		return nil, fmt.Errorf("%w: larger than 16 KB", ErrUnsupported)
	}
	root := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, fmt.Errorf("%w: not a JSON object", ErrUnsupported)
		}
	}
	if t, ok := root["type"]; ok && t != "object" {
		return nil, fmt.Errorf("%w: the root must be an object", ErrUnsupported)
	}
	root["type"] = "object"
	root["additionalProperties"] = false
	if _, ok := root["properties"]; !ok {
		root["properties"] = map[string]any{}
	}

	count := 0
	if err := check(root, "", 0, &count); err != nil {
		return nil, err
	}
	out, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode schema: %w", err)
	}
	return out, nil
}

/*
check walks one schema node, counting properties as it goes.
*/
func check(node map[string]any, at string, depth int, count *int) error {
	if depth > maxDepth {
		return fmt.Errorf("%w: %s nests deeper than %d levels", ErrUnsupported, pointer(at), maxDepth)
	}
	for key := range node {
		if !slices.Contains(allowedKeywords(), key) {
			return fmt.Errorf("%w: keyword %q at %s", ErrUnsupported, key, pointer(at))
		}
	}
	if err := checkScalars(node, at); err != nil {
		return err
	}
	if err := checkProperties(node, at, depth, count); err != nil {
		return err
	}
	if items, ok := node["items"]; ok {
		child, isObject := items.(map[string]any)
		if !isObject {
			return fmt.Errorf("%w: items at %s must be one schema", ErrUnsupported, pointer(at))
		}
		return check(child, at+"/items", depth+1, count)
	}
	return nil
}

/*
checkScalars checks the keywords that hold plain values.
*/
func checkScalars(node map[string]any, at string) error {
	if err := checkKinds(node, at); err != nil {
		return err
	}
	return checkBounds(node, at)
}

/*
checkKinds checks the type, format and pattern.
*/
func checkKinds(node map[string]any, at string) error {
	if t, ok := node["type"]; ok {
		name, isText := t.(string)
		if !isText || !slices.Contains(allowedTypes(), name) {
			return fmt.Errorf("%w: type at %s", ErrUnsupported, pointer(at))
		}
	}
	if f, ok := node["format"].(string); ok && !slices.Contains(allowedFormats(), f) {
		return fmt.Errorf("%w: format %q at %s", ErrUnsupported, f, pointer(at))
	}
	if p, ok := node["pattern"].(string); ok {
		if _, err := regexp.Compile(p); err != nil || len(p) > maxPattern {
			return fmt.Errorf("%w: pattern at %s", ErrUnsupported, pointer(at))
		}
	}
	return nil
}

/*
checkBounds checks the enum, description, list and object bounds.
*/
func checkBounds(node map[string]any, at string) error {
	if e, ok := node["enum"].([]any); ok && len(e) > maxEnum {
		return fmt.Errorf("%w: more than %d enum values at %s", ErrUnsupported, maxEnum, pointer(at))
	}
	if d, ok := node["description"].(string); ok && len([]rune(d)) > maxDescription {
		return fmt.Errorf("%w: description at %s is too long", ErrUnsupported, pointer(at))
	}
	if n, ok := node["maxItems"].(float64); ok && n > maxItems {
		return fmt.Errorf("%w: maxItems at %s above %d", ErrUnsupported, pointer(at), maxItems)
	}
	if ap, ok := node["additionalProperties"]; ok && ap != false {
		return fmt.Errorf("%w: additionalProperties at %s must be false", ErrUnsupported, pointer(at))
	}
	return nil
}

/*
checkProperties checks an object's properties and required list.
*/
func checkProperties(node map[string]any, at string, depth int, count *int) error {
	raw, ok := node["properties"]
	if !ok {
		return nil
	}
	props, isObject := raw.(map[string]any)
	if !isObject {
		return fmt.Errorf("%w: properties at %s", ErrUnsupported, pointer(at))
	}
	for name, p := range props {
		*count++
		if *count > maxProperties {
			return fmt.Errorf("%w: more than %d properties", ErrUnsupported, maxProperties)
		}
		child, isSchema := p.(map[string]any)
		if !isSchema {
			return fmt.Errorf("%w: property %s", ErrUnsupported, pointer(at+"/properties/"+name))
		}
		if err := check(child, at+"/properties/"+name, depth+1, count); err != nil {
			return err
		}
	}
	required, _ := node["required"].([]any)
	for _, r := range required {
		name, isText := r.(string)
		if _, declared := props[name]; !isText || !declared {
			return fmt.Errorf("%w: required %v is not a property at %s", ErrUnsupported, r, pointer(at))
		}
	}
	return nil
}

/*
pointer writes a location as a JSON pointer, "/" for the root.
*/
func pointer(at string) string {
	if at == "" {
		return "/"
	}
	return at
}
