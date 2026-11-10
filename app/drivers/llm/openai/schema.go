/*
|--------------------------------------------------------------------------
| Strict Response Schemas
|--------------------------------------------------------------------------
|
| OpenAI enforces a response schema only in strict mode, and strict
| mode refuses any object that stays open or leaves a property
| optional. The schema is checked here before a request leaves, so a
| caller learns of a loose schema as a bad request with the offending
| path, never as a vague provider refusal. The caller's schema is sent
| exactly as written; nothing here rewrites it.
|
*/

package openai

import (
	"encoding/json"
	"fmt"
	"slices"

	oai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
validName reports whether a response schema's name has the shape OpenAI
accepts: one to 64 letters, digits, underscores or dashes.
*/
func validName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		ok := r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}

/*
StrictSchema decodes a response schema and checks that strict mode
accepts it: a valid name, a JSON object, and every object in it closed
with additionalProperties false and all of its properties required. A
schema that breaks a rule refuses the request as invalid.
*/
func StrictSchema(rs *contracts.ResponseSchema) (map[string]any, error) {
	if !validName(rs.Name) {
		return nil, invalidSchema(rs.Name, "needs a name of letters, digits, _ or -")
	}

	var schema map[string]any
	if err := json.Unmarshal(rs.Schema, &schema); err != nil {
		return nil, invalidSchema(rs.Name, "is not a JSON object")
	}

	if problem := strictProblem(schema, "$"); problem != "" {
		return nil, invalidSchema(rs.Name, problem)
	}
	return schema, nil
}

/*
invalidSchema is the bad-request error for a schema strict mode
refuses.
*/
func invalidSchema(name, problem string) error {
	return &contracts.LLMError{
		Kind:         contracts.LLMErrorInvalidRequest,
		ProviderCode: "invalid_response_schema",
		Message:      "response schema " + name + " " + problem,
	}
}

/*
strictProblem walks one schema node and its children and describes the
first rule strict mode would refuse, or returns empty when there is
none. A node is an object when it says so or declares properties.
*/
func strictProblem(node map[string]any, path string) string {
	props, _ := node["properties"].(map[string]any)
	if node["type"] == "object" || props != nil || slices.Contains(types(node), "object") {
		if problem := objectProblem(node, props, path); problem != "" {
			return problem
		}
	}

	for _, child := range children(node, path) {
		if problem := strictProblem(child.node, child.path); problem != "" {
			return problem
		}
	}
	return ""
}

/*
objectProblem checks the two strict rules of one object node.
*/
func objectProblem(node, props map[string]any, path string) string {
	if closed, ok := node["additionalProperties"].(bool); !ok || closed {
		return "must set additionalProperties to false at " + path
	}

	required := stringList(node["required"])
	for name := range props {
		if !slices.Contains(required, name) {
			return fmt.Sprintf("must list %q as required at %s", name, path)
		}
	}
	return ""
}

/*
child is one nested schema and where it sits.
*/
type child struct {
	node map[string]any
	path string
}

/*
children lists the schemas nested in a node: its properties, its items,
its alternatives and its definitions.
*/
func children(node map[string]any, path string) []child {
	var out []child
	for _, key := range []string{"properties", "$defs", "definitions"} {
		named, _ := node[key].(map[string]any)
		for name, sub := range named {
			if m, ok := sub.(map[string]any); ok {
				out = append(out, child{node: m, path: path + "." + name})
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		out = append(out, child{node: items, path: path + "[]"})
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf"} {
		list, _ := node[key].([]any)
		for i, sub := range list {
			if m, ok := sub.(map[string]any); ok {
				out = append(out, child{node: m, path: fmt.Sprintf("%s.%s[%d]", path, key, i)})
			}
		}
	}
	return out
}

/*
types reads a node's type when it is a list of types.
*/
func types(node map[string]any) []string {
	return stringList(node["type"])
}

/*
stringList reads a JSON array of strings, skipping anything else.
*/
func stringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

/*
responseFormat maps a response schema to strict json_schema, refusing
a model the connection cannot enforce a schema on.
*/
func (d *Driver) responseFormat(req contracts.ChatRequest) (oai.ChatCompletionNewParamsResponseFormatUnion, error) {
	var format oai.ChatCompletionNewParamsResponseFormatUnion
	if !d.options.Capabilities(req.Model).JSONSchema {
		return format, fmt.Errorf("model %s: %w", req.Model, contracts.ErrSchemaUnsupported)
	}

	schema, err := StrictSchema(req.Schema)
	if err != nil {
		return format, err
	}

	format.OfJSONSchema = &shared.ResponseFormatJSONSchemaParam{
		JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
			Name: req.Schema.Name, Strict: oai.Bool(true), Schema: schema,
		},
	}
	return format, nil
}
