/*
|--------------------------------------------------------------------------
| Operations From An API Document
|--------------------------------------------------------------------------
|
| An application that already describes its HTTP API as an OpenAPI
| document can offer every operation as a tool without writing each one
| by hand. Each operation becomes one flat set of arguments: the path
| and query parameters and the fields of the JSON body side by side, so
| an agent fills in one object the way it reads one schema. The same
| operation turns those arguments back into the request the API expects.
| When the document changes, the operations change with it, so the tools
| never drift from the API.
|
*/

package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
)

/*
ErrInvalidDocument is returned for an API document that cannot be read.
*/
var ErrInvalidDocument = errors.New("mcp: invalid api document")

/*
bodyArgument holds the body when one of its fields shares a name with
a parameter, so neither hides the other.
*/
const bodyArgument = "request_body"

/*
maxDepth bounds how deep references are followed, so a schema that
refers to itself ends in a plain object instead of looping.
*/
const maxDepth = 8

/*
Operation is one operation of an API document, ready to become a tool.

ID is its operation id, Method and Path its HTTP method and path
template, Summary its one-line summary, and Abilities what a caller
must hold, read from the x-abilities extension. Input is the JSON schema
of its flat arguments, every reference followed. Upload reports that it
takes a file as a multipart form rather than JSON, which these
arguments cannot carry.
*/
type Operation struct {
	ID        string
	Method    string
	Path      string
	Summary   string
	Abilities []string
	Input     map[string]any
	Upload    bool
	params    []parameter
	body      bool
	nested    bool
}

/*
parameter is a path or query parameter of an operation.
*/
type parameter struct {
	name string
	in   string
}

/*
Operations reads the operations of an OpenAPI document, ordered by
operation id, keeping those keep accepts.
*/
func Operations(doc []byte, keep func(Operation) bool) ([]Operation, error) {
	var spec map[string]any
	if err := json.Unmarshal(doc, &spec); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDocument, err)
	}

	components, _ := asObject(spec["components"])["schemas"].(map[string]any)
	r := resolver{schemas: components}

	var out []Operation

	for path, item := range asObject(spec["paths"]) {
		for method, raw := range asObject(item) {
			op := asObject(raw)
			id, _ := op["operationId"].(string)

			if id == "" {
				continue
			}

			operation := r.operation(id, strings.ToUpper(method), path, op)
			if keep == nil || keep(operation) {
				out = append(out, operation)
			}
		}
	}

	slices.SortFunc(out, func(a, b Operation) int { return strings.Compare(a.ID, b.ID) })

	return out, nil
}

/*
Encoded is an operation's request built from arguments: its path with
the parameters filled in, its query, and its JSON body, nil when the
operation takes none.
*/
type Encoded struct {
	Path  string
	Query url.Values
	Body  []byte
}

/*
Encode turns arguments into the request the operation expects.
*/
func (o Operation) Encode(args map[string]any) (Encoded, error) {
	rest := maps.Clone(args)
	e := Encoded{Path: o.Path, Query: url.Values{}}

	for _, p := range o.params {
		value, ok := rest[p.name]
		delete(rest, p.name)

		if !ok || value == nil {
			if p.in == "path" {
				return Encoded{}, fmt.Errorf("missing argument %q", p.name)
			}

			continue
		}

		if p.in == "path" {
			e.Path = strings.ReplaceAll(e.Path, "{"+p.name+"}", url.PathEscape(text(value)))

			continue
		}

		for _, v := range listOf(value) {
			e.Query.Add(p.name, text(v))
		}
	}

	if !o.body {
		return e, nil
	}

	if o.nested {
		rest, _ = rest[bodyArgument].(map[string]any)
	}

	body, err := json.Marshal(nonNil(rest))
	if err != nil {
		return Encoded{}, fmt.Errorf("encode body: %w", err)
	}

	e.Body = body

	return e, nil
}

/*
resolver follows references into the document's component schemas.
*/
type resolver struct {
	schemas map[string]any
}

/*
operation builds one operation's flat input schema.
*/
func (r resolver) operation(id, method, path string, op map[string]any) Operation {
	o := Operation{ID: id, Method: method, Path: path}
	o.Summary, _ = op["summary"].(string)

	for _, ability := range asList(op["x-abilities"]) {
		if s, ok := ability.(string); ok {
			o.Abilities = append(o.Abilities, s)
		}
	}

	properties, required := r.parameters(&o, op)

	content := asObject(asObject(op["requestBody"])["content"])
	o.Upload = len(content) > 0 && content["application/json"] == nil

	body := r.body(op)
	if body != nil {
		o.body = true
		properties, required, o.nested = merge(properties, required, body)
	}

	o.Input = map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		o.Input["required"] = required
	}

	return o
}

/*
parameters collects an operation's path and query parameters as
arguments, with those that must be given.
*/
func (r resolver) parameters(o *Operation, op map[string]any) (map[string]any, []any) {
	properties := map[string]any{}
	var required []any

	for _, raw := range asList(op["parameters"]) {
		p := asObject(raw)
		name, _ := p["name"].(string)
		in, _ := p["in"].(string)

		if in != "path" && in != "query" {
			continue
		}

		schema := r.inline(asObject(p["schema"]), 0)
		if description, ok := p["description"].(string); ok && schema["description"] == nil {
			schema["description"] = description
		}

		properties[name] = schema
		o.params = append(o.params, parameter{name: name, in: in})

		if in == "path" || p["required"] == true {
			required = append(required, name)
		}
	}

	return properties, required
}

/*
body returns the inlined JSON body schema of an operation, nil when it
takes no JSON body.
*/
func (r resolver) body(op map[string]any) map[string]any {
	content := asObject(asObject(op["requestBody"])["content"])
	media := asObject(content["application/json"])

	if media == nil {
		return nil
	}

	return r.inline(asObject(media["schema"]), 0)
}

/*
merge puts the body's fields beside the parameters, or the whole body
under one argument when a field would hide a parameter.
*/
func merge(properties map[string]any, required []any, body map[string]any) (map[string]any, []any, bool) {
	fields := asObject(body["properties"])
	for name := range fields {
		if _, clash := properties[name]; clash {
			properties[bodyArgument] = body
			if len(asList(body["required"])) > 0 {
				required = append(required, bodyArgument)
			}

			return properties, required, true
		}
	}

	maps.Copy(properties, fields)
	required = append(required, asList(body["required"])...)

	return properties, required, false
}

/*
inline copies a schema with every reference replaced by the schema it
names, down to a bounded depth.
*/
func (r resolver) inline(schema map[string]any, depth int) map[string]any {
	if schema == nil {
		return map[string]any{}
	}

	if ref, ok := schema["$ref"].(string); ok {
		if depth >= maxDepth {
			return map[string]any{"type": "object"}
		}

		target := asObject(r.schemas[strings.TrimPrefix(ref, "#/components/schemas/")])

		return r.inline(target, depth+1)
	}

	out := make(map[string]any, len(schema))

	for key, value := range schema {
		if key == "$schema" {
			continue
		}

		out[key] = r.value(value, depth)
	}

	if properties := asObject(out["properties"]); properties != nil {
		delete(properties, "$schema")
	}

	return out
}

/*
value inlines the schemas nested in a schema keyword's value.
*/
func (r resolver) value(value any, depth int) any {
	switch v := value.(type) {
	case map[string]any:
		if _, isRef := v["$ref"]; isRef || isSchema(v) {
			return r.inline(v, depth)
		}

		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = r.value(item, depth)
		}

		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = r.value(item, depth)
		}

		return out
	default:
		return v
	}
}

/*
isSchema reports whether an object looks like a schema rather than a
map of named schemas.
*/
func isSchema(v map[string]any) bool {
	for _, key := range []string{"type", "properties", "items", "enum", "oneOf", "anyOf", "allOf"} {
		if _, ok := v[key]; ok {
			return true
		}
	}

	return false
}

/*
text writes an argument the way a URL carries it.
*/
func text(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%f", value), "0"), ".")
	default:
		data, _ := json.Marshal(value)

		return strings.Trim(string(data), `"`)
	}
}

/*
listOf returns a list argument's items, or the argument alone.
*/
func listOf(v any) []any {
	if list, ok := v.([]any); ok {
		return list
	}

	return []any{v}
}

/*
nonNil returns an empty object for a missing body.
*/
func nonNil(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}

	return m
}

/*
asObject reads a JSON object, nil for anything else.
*/
func asObject(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}

/*
asList reads a JSON array, nil for anything else.
*/
func asList(v any) []any {
	l, _ := v.([]any)

	return l
}
