/*
|--------------------------------------------------------------------------
| Input Constraints In The Document
|--------------------------------------------------------------------------
|
| Request types state their rules once, in validate tags, and the
| validator enforces them. The published document repeats those rules as
| JSON Schema, so a client or an agent reading it knows which fields are
| required, which values are allowed and how long a text may be before
| it sends anything. Only the published copy changes: the schemas the
| API layer validates with stay as they are, so requests are judged the
| same way whether the document says more or not.
|
*/

package routing

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

/*
document is a published API document being annotated.
*/
type document struct {
	paths   map[string]any
	schemas map[string]any
	seen    map[reflect.Type]bool
}

/*
constrain adds the validate rules of each route's input to the published
document of the API it belongs to.
*/
func constrain(raw []byte, spec *apiSpec, routes []*Route) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("read openapi %s: %w", spec.prefix, err)
	}

	d := document{paths: object(doc["paths"]), seen: map[reflect.Type]bool{}}
	if components := object(doc["components"]); components != nil {
		d.schemas = object(components["schemas"])
	}

	for _, route := range routes {
		if route.api == spec {
			d.route(route)
		}
	}

	out, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode openapi %s: %w", spec.prefix, err)
	}

	return out, nil
}

/*
route annotates one operation: its parameters, then its body.
*/
func (d document) route(route *Route) {
	op := object(object(d.paths[route.operation().Path])[strings.ToLower(route.method)])
	if op == nil {
		return
	}

	d.queries(op, route.queries)

	input := route.input
	if input == nil {
		return
	}

	for input.Kind() == reflect.Pointer {
		input = input.Elem()
	}

	if input.Kind() != reflect.Struct {
		return
	}

	for field := range input.Fields() {
		if field.Name == "Body" {
			d.body(op, field.Type)

			continue
		}

		d.parameter(op, field)
	}
}

/*
queries adds the query parameters a handler reads itself, unless the
operation already documents one of the same name.
*/
func (d document) queries(op map[string]any, params []QueryParam) {
	if len(params) == 0 {
		return
	}

	list, _ := op["parameters"].([]any)
	known := map[string]bool{}

	for _, p := range list {
		if name, ok := object(p)["name"].(string); ok {
			known[name] = true
		}
	}

	for _, p := range params {
		if known[p.Name] {
			continue
		}

		schema := map[string]any{"type": "string"}
		if len(p.Values) > 0 {
			values := make([]any, len(p.Values))
			for i, v := range p.Values {
				values[i] = v
			}

			schema["enum"] = values
		}

		list = append(list, map[string]any{
			"in": "query", "name": p.Name, "description": p.Description, "schema": schema,
		})
	}

	op["parameters"] = list
}

/*
parameter annotates the path, query or header parameter a field binds.
*/
func (d document) parameter(op map[string]any, field reflect.StructField) {
	name, in := "", ""
	for _, tag := range []string{"path", "query", "header"} {
		if v := field.Tag.Get(tag); v != "" {
			name, in = strings.Split(v, ",")[0], tag
		}
	}

	if name == "" {
		return
	}

	params, _ := op["parameters"].([]any)
	for _, p := range params {
		param := object(p)
		if param["name"] != name || param["in"] != in {
			continue
		}

		rules := parseRules(field.Tag.Get("validate"))
		if schema := object(param["schema"]); schema != nil {
			rules.apply(schema, field.Type)
		}

		if rules.required {
			param["required"] = true
		}
	}
}

/*
body annotates the request body schema with the body type's rules.
*/
func (d document) body(op map[string]any, t reflect.Type) {
	content := object(object(op["requestBody"])["content"])
	schema := object(object(content["application/json"])["schema"])
	d.structure(d.resolve(schema), t)
}

/*
structure applies the rules of a struct's fields to its object schema,
and of nested structs to theirs, each type once.
*/
func (d document) structure(schema map[string]any, t reflect.Type) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	if schema == nil || t.Kind() != reflect.Struct || d.seen[t] {
		return
	}

	d.seen[t] = true
	properties := object(schema["properties"])

	for field := range t.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")

		if name == "" || name == "-" || properties == nil {
			continue
		}

		property := object(properties[name])
		if property == nil {
			continue
		}

		rules := parseRules(field.Tag.Get("validate"))
		rules.apply(d.resolve(property), field.Type)

		if rules.required {
			schema["required"] = appendOnce(schema["required"], name)
		}

		nested := d.resolve(property)
		if items := object(nested["items"]); items != nil {
			nested = d.resolve(items)
		}

		d.structure(nested, field.Type)
	}
}

/*
resolve follows a schema's reference to the component it names, or
returns the schema itself.
*/
func (d document) resolve(schema map[string]any) map[string]any {
	ref, _ := schema["$ref"].(string)
	if ref == "" || d.schemas == nil {
		return schema
	}

	return object(d.schemas[strings.TrimPrefix(ref, "#/components/schemas/")])
}

/*
rules are the parts of a validate tag the document can state: whether
the field is required, its allowed values, its bounds and its format,
and the rules for the items of a list.
*/
type rules struct {
	required bool
	enum     []string
	min, max *float64
	format   string
	items    *rules
}

/*
parseRules reads a validate tag. Rules after dive describe the items of
a list.
*/
func parseRules(tag string) rules {
	var r rules

	parts := strings.Split(tag, ",")
	for i, part := range parts {
		key, value, _ := strings.Cut(part, "=")

		switch key {
		case "dive":
			items := parseRules(strings.Join(parts[i+1:], ","))
			r.items = &items

			return r
		case "required":
			r.required = true
		case "oneof":
			r.enum = strings.Fields(value)
		case "max", "lte":
			r.max = number(value)
		case "min", "gte":
			r.min = number(value)
		case "len":
			r.min, r.max = number(value), number(value)
		case "email":
			r.format = "email"
		case "url":
			r.format = "uri"
		}
	}

	return r
}

/*
apply writes the rules into a property's schema, as bounds on length,
value or count by the field's kind.
*/
func (r rules) apply(schema map[string]any, t reflect.Type) {
	if schema == nil {
		return
	}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if len(r.enum) > 0 {
		schema["enum"] = enumValues(r.enum, t.Kind())
	}

	if r.format != "" {
		schema["format"] = r.format
	}

	low, high := bounds(t.Kind())
	if r.min != nil {
		schema[low] = *r.min
	}

	if r.max != nil {
		schema[high] = *r.max
	}

	if r.items != nil && t.Kind() == reflect.Slice {
		if items := object(schema["items"]); items != nil {
			r.items.apply(items, t.Elem())
		}
	}
}

/*
bounds names the schema keywords a field's kind is bounded by.
*/
func bounds(kind reflect.Kind) (string, string) {
	if kind == reflect.String {
		return "minLength", "maxLength"
	}

	if kind == reflect.Slice || kind == reflect.Array {
		return "minItems", "maxItems"
	}

	if kind == reflect.Map {
		return "minProperties", "maxProperties"
	}

	return "minimum", "maximum"
}

/*
enumValues types the allowed values as the field holds them.
*/
func enumValues(values []string, kind reflect.Kind) []any {
	out := make([]any, 0, len(values))

	for _, v := range values {
		if n := number(v); n != nil && kind != reflect.String {
			out = append(out, *n)

			continue
		}

		out = append(out, v)
	}

	return out
}

/*
number reads a rule's numeric argument, nil when it is not a number.
*/
func number(value string) *float64 {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}

	return &n
}

/*
appendOnce adds a name to a JSON list unless it is already there.
*/
func appendOnce(list any, name string) []any {
	items, _ := list.([]any)
	for _, item := range items {
		if item == name {
			return items
		}
	}

	return append(items, name)
}

/*
object reads a JSON object, nil for anything else.
*/
func object(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}
