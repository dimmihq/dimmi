/*
|--------------------------------------------------------------------------
| Tools From Operations
|--------------------------------------------------------------------------
|
| Each API operation becomes a tool named after it, in the words agents
| expect: list, get, create, update, delete. Its risk follows from what
| it does unless the catalog says otherwise: reads are reads, deletes and
| changes to keys, members, roles and settings are dangerous. Its
| description is the catalog's, with the risk said plainly. Its
| arguments carry the names a model reads right, and every one is
| explained, with where to find the id it needs or what a value looks
| like.
|
*/

package mcpserver

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
confirmArgument is the argument that carries the person's approval of a
tool that needs it, for clients that cannot ask the person themselves.
*/
const confirmArgument = "confirmation_token"

/*
dangerousAbilities are abilities whose writes change how the workspace
works or who is in it.
*/
func dangerousAbilities() []string {
	return []string{"settings:write", "members:write", "ai:keys", "roles:write", "billing:write"}
}

/*
riskOf returns how careful an agent must be with an operation: the
catalog's word when it has one, else by method and abilities.
*/
func riskOf(op mcp.Operation, doc Doc) Risk {
	if doc.Risk != nil {
		return *doc.Risk
	}

	if op.Method == fhttp.MethodGet {
		return RiskRead
	}

	if op.Method == fhttp.MethodDelete {
		return RiskDanger
	}

	for _, ability := range op.Abilities {
		if slices.Contains(dangerousAbilities(), ability) {
			return RiskDanger
		}
	}

	return RiskWrite
}

/*
confirms reports whether the person approves a call before it runs.
*/
func (r Risk) confirms() bool {
	return r >= RiskSensitive
}

/*
caution is the sentence a description ends with for a risk.
*/
func (r Risk) caution() string {
	switch r {
	case RiskRead:
		return "Read-only."
	case RiskWrite:
		return "Changes data only your team sees."
	case RiskSensitive:
		return "Reads private data: the person must approve each call."
	case RiskReach:
		return "Reaches customers or the public: the person must approve each call."
	case RiskDanger:
		return "Hard to undo or changes how the workspace works: the person must approve each call."
	default:
		return ""
	}
}

/*
built is one operation as a tool: its catalog entry, its definition,
how its arguments are named, the schema the model fills, the texts
written for its arguments and the tool that stands for each endpoint
the API document names.
*/
type built struct {
	op      mcp.Operation
	doc     Doc
	def     mcp.ToolDef
	names   naming
	schema  map[string]any
	written map[string]string
	tools   map[string]string
}

/*
build makes the tool of an operation.
*/
func build(op mcp.Operation, doc Doc, tools map[string]string) built {
	b := built{op: op, doc: doc, names: namingOf(op), tools: tools}
	b.written = writtenTexts()[strings.TrimPrefix(toolName(op, doc), toolPrefix)]
	b.schema = b.inputSchema()
	b.def = b.definition()
	b.def.Arguments, b.def.Invalid = b.read, b.invalid

	return b
}

/*
definition builds the tool definition of an operation.
*/
func (b built) definition() mcp.ToolDef {
	op, doc := b.op, b.doc
	r := riskOf(op, doc)

	return mcp.ToolDef{
		Name:        toolName(op, doc),
		Title:       doc.Title,
		Description: describe(doc, r),
		Abilities:   op.Abilities,
		Confirm:     r.confirms(),
		ReadOnly:    op.Method == fhttp.MethodGet && r == RiskRead,
		Destructive: op.Method == fhttp.MethodDelete || r == RiskDanger,
		Idempotent:  op.Method == fhttp.MethodGet || op.Method == fhttp.MethodPut || op.Method == fhttp.MethodDelete,
		OpenWorld:   r == RiskReach,
	}
}

/*
describe writes a tool's description: what it does and when to use it,
what it returns, and how careful to be.
*/
func describe(doc Doc, r Risk) string {
	return purpose(doc) + " " + r.caution()
}

/*
purpose says what an operation does, when to use it and what it
returns, the same for the tool and the endpoint.
*/
func purpose(doc Doc) string {
	parts := []string{strings.TrimSpace(doc.Use)}
	if doc.Returns != "" {
		parts = append(parts, "Returns "+strings.TrimSpace(doc.Returns))
	}

	return strings.Join(parts, " ")
}

/*
inputSchema copies an operation's input schema with every argument
explained in the API document's own words, an endpoint it names said
as the tool that stands for it, and the approval argument added to
tools that need one. An argument the document says nothing of is
explained the way the document would be.
*/
func (b built) inputSchema() map[string]any {
	input := maps.Clone(b.op.Input)
	fields := asMap(input["properties"])
	properties := map[string]any{}

	for name := range b.names.api {
		schema := b.describeFields(maps.Clone(asMap(fields[name])), name)
		if text, _ := schema["description"].(string); text != "" && b.tools != nil {
			schema["description"] = toolWords(text, b.tools)
		} else {
			schema["description"] = b.argumentText(name, schema)
		}

		properties[name] = schema
	}

	if riskOf(b.op, b.doc).confirms() {
		properties[confirmArgument] = map[string]any{
			"type": "string",
			"description": "Leave out on the first call. When the result asks for the person's approval and they " +
				"give it, call again with the same arguments and this token from that result.",
		}
	}

	input["properties"] = properties

	return input
}

/*
describeFields returns a schema with the fields inside it explained:
in the API document's words, else by the text written for the field's
path from the argument, such as contact.email, or edits.find for a
field of each item of a list. Only what it changes is copied.
*/
func (b built) describeFields(schema map[string]any, path string) map[string]any {
	if fields := asMap(schema["properties"]); fields != nil {
		described := make(map[string]any, len(fields))

		for name, raw := range fields {
			field := asMap(raw)
			if field == nil {
				described[name] = raw

				continue
			}

			at := path + "." + name
			field = b.describeFields(maps.Clone(field), at)

			if text, _ := field["description"].(string); text != "" {
				field["description"] = toolWords(text, b.tools)
			} else if text, ok := b.written[at]; ok {
				field["description"] = text
			}

			described[name] = field
		}

		schema["properties"] = described
	}

	if items := asMap(schema["items"]); items != nil {
		schema["items"] = b.describeFields(maps.Clone(items), path)
	}

	return schema
}

/*
required lists the arguments a call must carry.
*/
func (b built) required() []string {
	var out []string
	for _, name := range asList(b.schema["required"]) {
		out = append(out, fmt.Sprint(name))
	}

	return out
}

/*
argumentText explains an argument: the catalog's words, else the words
written for it by tool and name, else what is known about the record a
path names or an argument every list shares, else the API document's
words, else what its name and type say.
*/
func (b built) argumentText(name string, schema map[string]any) string {
	if text, ok := b.doc.Params[name]; ok {
		return text
	}

	if text, ok := b.written[name]; ok {
		return text
	}

	if slices.Contains(pathArguments(b.op.Path), name) {
		if guide, ok := idGuides()[name]; ok {
			return guide.String()
		}
	}

	if text, ok := listArguments()[name]; ok {
		return text
	}

	if text, _ := schema["description"].(string); text != "" {
		return text
	}

	if field, ok := strings.CutPrefix(name, "filter["); ok {
		return "Only items whose " + strings.TrimSuffix(field, "]") + " matches. Several values separated by " +
			"commas match any of them."
	}

	if text := knownText(b.op, name, schema); text != "" {
		return text
	}

	return plainText(name, schema)
}

/*
plainText explains an argument nobody wrote about, from its name and
what its schema allows, with what a value looks like.
*/
func plainText(name string, schema map[string]any) string {
	label := strings.ReplaceAll(name, "_", " ")
	label = strings.ToUpper(label[:1]) + label[1:]

	if values := asList(schema["enum"]); len(values) > 0 {
		words := make([]string, 0, len(values))
		for _, v := range values {
			if text := fmt.Sprint(v); text != "" {
				words = append(words, text)
			}
		}

		return label + ": one of " + strings.Join(words, ", ") + "."
	}

	return label + ": " + valueText(name, schema)
}

/*
valueText says what a value of an argument looks like, from its type.
*/
func valueText(name string, schema map[string]any) string {
	kind := kindOf(schema)
	format, _ := schema["format"].(string)

	switch {
	case kind == "boolean":
		return "true or false."
	case kind == "integer" || kind == "number":
		return "a number, such as " + sample(schema) + "."
	case kind == "array":
		return "a list, such as [\"...\"]."
	case kind == "object":
		return "a JSON object."
	case format == "date-time":
		return "a time, such as 2026-01-31T09:00:00Z."
	case format == "date":
		return "a date, such as 2026-01-31."
	case format == "email":
		return "an email address, such as ana@example.com."
	case format == "uri":
		return "a URL, such as https://example.com."
	case strings.HasSuffix(name, "_id"):
		return "the id of the " + strings.ReplaceAll(strings.TrimSuffix(name, "_id"), "_", " ") +
			", as the tool that lists them shows it."
	}

	if limit, ok := schema["maxLength"].(float64); ok {
		return fmt.Sprintf("text of at most %d characters.", int(limit))
	}

	return "text."
}

/*
sample is a number an argument's schema allows.
*/
func sample(schema map[string]any) string {
	if low, ok := schema["minimum"].(float64); ok && low > 0 {
		return fmt.Sprint(int(low))
	}

	return "10"
}

/*
asMap reads a JSON object, nil for anything else.
*/
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}
