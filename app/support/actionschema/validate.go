/*
|--------------------------------------------------------------------------
| Argument Validation
|--------------------------------------------------------------------------
|
| The model's arguments are checked against the admin's schema before a
| single byte is sent: invalid arguments never reach a customer's API.
| Strings the admin left unbounded are capped at 200 characters, so a
| prompt-injected customer cannot stuff a huge value into a request.
|
*/

package actionschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
)

/*
defaultMaxLength caps a string property the schema left unbounded.
*/
const defaultMaxLength = 200

/*
ErrInvalidArguments marks arguments the schema refuses.
*/
var ErrInvalidArguments = errors.New("invalid arguments")

/*
Schema is a compiled input schema, ready to validate arguments.
*/
type Schema struct {
	resolved *jsonschema.Resolved
}

/*
Compile resolves a normalized schema for validation.
*/
func Compile(raw json.RawMessage) (*Schema, error) {
	tree := map[string]any{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	capStrings(tree)
	data, err := json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("encode schema: %w", err)
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	resolved, err := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	return &Schema{resolved: resolved}, nil
}

/*
Validate checks the arguments, fills in the schema's defaults and
returns them decoded. Missing arguments are an empty object.
*/
func (s *Schema) Validate(args json.RawMessage) (map[string]any, error) {
	out := map[string]any{}
	if len(args) > 0 && string(args) != "null" {
		if err := json.Unmarshal(args, &out); err != nil {
			return nil, fmt.Errorf("%w: arguments must be a JSON object", ErrInvalidArguments)
		}
	}
	if err := s.resolved.ApplyDefaults(&out); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArguments, err)
	}
	if err := s.resolved.Validate(out); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidArguments, err)
	}
	return out, nil
}

/*
Properties lists the top-level properties a schema declares.
*/
func Properties(raw json.RawMessage) []string {
	var root struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if json.Unmarshal(raw, &root) != nil {
		return nil
	}
	out := make([]string, 0, len(root.Properties))
	for name := range root.Properties {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

/*
Without returns the schema with the named top-level properties removed,
from properties and required alike: what the model sees once the admin
bound those arguments itself.
*/
func Without(raw json.RawMessage, names []string) json.RawMessage {
	if len(names) == 0 {
		return raw
	}
	root := map[string]any{}
	if json.Unmarshal(raw, &root) != nil {
		return raw
	}
	if props, ok := root["properties"].(map[string]any); ok {
		for _, n := range names {
			delete(props, n)
		}
	}
	if required, ok := root["required"].([]any); ok {
		kept := []any{}
		for _, r := range required {
			if name, _ := r.(string); !slices.Contains(names, name) {
				kept = append(kept, r)
			}
		}
		root["required"] = kept
	}
	out, err := json.Marshal(root)
	if err != nil {
		return raw
	}
	return out
}

/*
capStrings adds the default length cap to every string schema without
one.
*/
func capStrings(node map[string]any) {
	if node["type"] == "string" {
		if _, ok := node["maxLength"]; !ok {
			node["maxLength"] = defaultMaxLength
		}
	}
	if props, ok := node["properties"].(map[string]any); ok {
		for _, p := range props {
			if child, isSchema := p.(map[string]any); isSchema {
				capStrings(child)
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		capStrings(items)
	}
}
