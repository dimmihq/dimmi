/*
|--------------------------------------------------------------------------
| Reading A Call's Arguments
|--------------------------------------------------------------------------
|
| A model sends what it thinks a tool takes, and a weak one is often
| nearly right: the API's name of an argument, a number where text is
| meant, one value where a list is. Each is read as what it plainly
| means. What cannot be read is refused before anything runs, in words
| the model can act on: which argument is missing and what belongs in
| it, which one the tool does not have and what it is likely called.
|
*/

package mcpserver

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

/*
refusedProperty finds where the schema names the argument it refused.
*/
var refusedProperty = regexp.MustCompile(`validating /properties/([^:/]+)[^:]*: `)

/*
read turns the arguments a model sent into the ones the tool takes, or
says what to fix.
*/
func (b built) read(args map[string]any) (map[string]any, error) {
	properties := asMap(b.schema["properties"])
	out := map[string]any{}
	problem := &argumentError{tool: b}

	for _, name := range slices.Sorted(maps.Keys(args)) {
		target, known := b.target(name, args)
		if !known {
			problem.unknown = append(problem.unknown, name)
		}

		if target != "" && args[name] != nil {
			out[target] = coerce(args[name], asMap(properties[target]))
			problem.wrong = append(problem.wrong, refuseAll(target, out[target], asMap(properties[target]))...)
		}
	}

	for _, name := range b.required() {
		if value, ok := out[name]; !ok || value == "" {
			problem.missing = append(problem.missing, name)
		}
	}

	if len(problem.unknown)+len(problem.missing)+len(problem.wrong) > 0 {
		return nil, problem
	}

	return out, nil
}

/*
target is the argument a sent name is read as: itself, or the argument
it is another name for unless that one was sent too. It reports whether
the tool knows the name at all.
*/
func (b built) target(name string, args map[string]any) (string, bool) {
	if _, ok := asMap(b.schema["properties"])[name]; ok {
		return name, true
	}

	alias, known := b.names.aliases[name]
	if _, sent := args[alias]; sent {
		return "", known
	}

	return alias, known
}

/*
invalid explains arguments the schema refused.
*/
func (b built) invalid(err error) error {
	reason := err.Error()
	for _, prefix := range []string{"validate arguments: ", "validating root: "} {
		reason = strings.TrimPrefix(reason, prefix)
	}

	reason = refusedProperty.ReplaceAllString(reason, `"$1": `)

	return &argumentError{tool: b, reason: reason}
}

/*
coerce reads a value as the type its argument takes when the model sent
it as another: a number or true for text, text for a number, one value
for a list.
*/
func coerce(value any, schema map[string]any) any {
	kind := kindOf(schema)

	switch v := value.(type) {
	case float64:
		if kind == "string" {
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
	case bool:
		if kind == "string" {
			return strconv.FormatBool(v)
		}
	case string:
		return fromText(v, kind)
	}

	if _, list := value.([]any); kind == "array" && !list {
		return []any{value}
	}

	return value
}

/*
refuseAll says in plain words everything wrong with a value and the
fields inside it: a value its argument does not take, a field an object
does not have, a field it must have and lacks.
*/
func refuseAll(path string, value any, schema map[string]any) []string {
	var out []string
	if wrong := refuse(path, value, schema); wrong != "" {
		out = append(out, wrong)
	}

	if items, list := value.([]any); list {
		for _, item := range items {
			out = append(out, refuseAll(path, item, asMap(schema["items"]))...)
		}

		return out
	}

	return append(out, refuseFields(path, asMap(value), schema)...)
}

/*
refuseFields says what is wrong with an object's fields: one it does
not have, one it must have and lacks, and what each one's value breaks.
*/
func refuseFields(path string, object, schema map[string]any) []string {
	fields := asMap(schema["properties"])
	if object == nil || fields == nil {
		return nil
	}

	var out []string

	for _, name := range slices.Sorted(maps.Keys(object)) {
		field, known := fields[name]
		if !known && schema["additionalProperties"] == false {
			out = append(out, fmt.Sprintf("%q is not a field of %q. Its fields: %s.", path+"."+name, path,
				strings.Join(slices.Sorted(maps.Keys(fields)), ", ")))
		}

		if known && object[name] != nil {
			out = append(out, refuseAll(path+"."+name, object[name], asMap(field))...)
		}
	}

	for _, required := range asList(schema["required"]) {
		name := fmt.Sprint(required)
		if _, sent := object[name]; !sent {
			text, _ := asMap(fields[name])["description"].(string)
			out = append(out, strings.TrimSpace(fmt.Sprintf("%q is required and missing. %s", path+"."+name, text)))
		}
	}

	return out
}

/*
refuse says in plain words why an argument does not take the value
sent, empty when it does: a value outside the few it allows, text too
long or too short, or a number out of range.
*/
func refuse(name string, value any, schema map[string]any) string {
	sent := fmt.Sprint(value)

	if allowed := asList(schema["enum"]); len(allowed) > 0 {
		words := make([]string, 0, len(allowed))
		for _, v := range allowed {
			if fmt.Sprint(v) != "" {
				words = append(words, fmt.Sprint(v))
			}
		}

		if !slices.Contains(words, sent) && sent != "" {
			return fmt.Sprintf("%q must be one of %s; you sent %q.", name, strings.Join(words, ", "), sent)
		}

		return ""
	}

	switch v := value.(type) {
	case string:
		return refuseText(name, v, schema)
	case float64:
		low, hasLow := schema["minimum"].(float64)
		high, hasHigh := schema["maximum"].(float64)

		if (hasLow && v < low) || (hasHigh && v > high) {
			return fmt.Sprintf("%q must be %s; you sent %s.", name, between(schema), sent)
		}
	}

	return ""
}

/*
refuseText says why text is too long or too short for an argument.
*/
func refuseText(name, v string, schema map[string]any) string {
	length := len([]rune(v))

	if limit, ok := schema["maxLength"].(float64); ok && length > int(limit) {
		return fmt.Sprintf("%q takes at most %d characters; you sent %d. Shorten it.", name, int(limit), length)
	}

	if least, ok := schema["minLength"].(float64); ok && length < int(least) {
		return fmt.Sprintf("%q needs at least %d characters; you sent %d.", name, int(least), length)
	}

	return ""
}

/*
between says the range a number must be in.
*/
func between(schema map[string]any) string {
	low, hasLow := schema["minimum"].(float64)
	high, hasHigh := schema["maximum"].(float64)

	switch {
	case hasLow && hasHigh:
		return fmt.Sprintf("from %v to %v", low, high)
	case hasLow:
		return fmt.Sprintf("%v or more", low)
	case hasHigh:
		return fmt.Sprintf("%v or less", high)
	default:
		return "a number"
	}
}

/*
fromText reads text as the number, switch or list its argument takes.
*/
func fromText(v, kind string) any {
	switch kind {
	case "integer", "number":
		if n, err := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(v), "#"), 64); err == nil {
			return n
		}
	case "boolean":
		if on, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return on
		}
	case "array":
		return []any{v}
	}

	return v
}

/*
argumentError is a call whose arguments do not fit its tool: arguments
it must carry and does not, arguments the tool does not have, values
an argument does not take, or anything else the schema refuses.
*/
type argumentError struct {
	tool    built
	missing []string
	unknown []string
	wrong   []string
	reason  string
}

/*
Error describes the refused call in one line.
*/
func (e *argumentError) Error() string {
	return e.tool.def.Name + " was called with arguments that do not fit"
}

/*
ToolResult writes the refusal for the agent: what is wrong with each
argument, what the tool takes, and what to do now.
*/
func (e *argumentError) ToolResult() (string, any) {
	var b strings.Builder
	name := e.tool.def.Name
	properties := asMap(e.tool.schema["properties"])

	fmt.Fprintf(&b, "%s was not run: fix the arguments and call %s again. Nothing was changed.\n",
		title(e.tool.doc, name), name)

	for _, arg := range e.missing {
		text, _ := asMap(properties[arg])["description"].(string)
		fmt.Fprintf(&b, "- %q is required and missing. %s\n", arg, text)
	}

	for _, arg := range e.unknown {
		fmt.Fprintf(&b, "- %q is not an argument of this tool.%s\n", arg, e.suggest(arg))
	}

	for _, wrong := range e.wrong {
		fmt.Fprintf(&b, "- %s\n", wrong)
	}

	if e.reason != "" {
		fmt.Fprintf(&b, "- %s\n", e.reason)
	}

	fmt.Fprintf(&b, "Arguments of %s: %s.\n", name, strings.Join(e.signature(), ", "))

	return b.String(), map[string]any{"error": map[string]any{
		"code": "invalid_arguments", "missing": e.missing, "unknown": e.unknown, "wrong": e.wrong,
		"reason":    e.reason,
		"arguments": e.signature(),
	}}
}

/*
signature lists the tool's arguments, the required ones marked.
*/
func (e *argumentError) signature() []string {
	required := e.tool.required()

	var out []string
	for _, name := range slices.Sorted(maps.Keys(asMap(e.tool.schema["properties"]))) {
		if name == confirmArgument {
			continue
		}

		if slices.Contains(required, name) {
			name += " (required)"
		}

		out = append(out, name)
	}

	if len(out) == 0 {
		return []string{"none"}
	}

	return out
}

/*
suggest names the argument an unknown one was most likely meant to be.
*/
func (e *argumentError) suggest(unknown string) string {
	for _, name := range e.tool.names.names() {
		if strings.Contains(name, unknown) || strings.Contains(unknown, name) {
			return fmt.Sprintf(" Did you mean %q?", name)
		}
	}

	return ""
}
