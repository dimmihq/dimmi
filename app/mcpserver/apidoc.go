/*
|--------------------------------------------------------------------------
| Describing The API Document
|--------------------------------------------------------------------------
|
| The words that teach an agent a tool are the words that explain the
| API, so they are written once and put into the API document itself:
| every operation says what it is for and what it returns, and every
| parameter and every field of a request, nested or not, says what it
| takes. The contract, the API reference and the SDKs are generated
| from that document, so they carry the same words the tools do. Where
| a text names a tool, the document names the endpoint instead.
|
*/

package mcpserver

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
schemaPrefix starts a reference to one of the document's schemas.
*/
const schemaPrefix = "#/components/schemas/"

/*
apiDocument is an API document being described: its schemas, the
catalog, the written texts and the endpoint each tool stands for.
*/
type apiDocument struct {
	schemas   map[string]any
	docs      map[string]Doc
	written   map[string]map[string]string
	endpoints map[string]string
	claims    map[string][]claim
	fieldOf   map[string]map[string]any
}

/*
DescribeAPI writes into the document of the API the tools are built
from what every operation is for and what every parameter and request
field takes. Other documents are left as they are.
*/
func DescribeAPI(prefix string, doc map[string]any) {
	if prefix != apiPrefix {
		return
	}

	d := apiDocument{
		schemas: asMap(asMap(doc["components"])["schemas"]), docs: catalog(), written: writtenTexts(),
		endpoints: map[string]string{}, claims: map[string][]claim{}, fieldOf: map[string]map[string]any{},
	}
	ops := d.operations(doc)

	for _, op := range ops {
		d.endpoints[toolName(op.operation, d.docs[op.operation.ID])] = op.operation.Method + " " + op.operation.Path
	}

	for _, op := range ops {
		d.describe(op)
	}

	d.settle()
	d.schemaFields()
}

/*
apiOperation is one operation of the document: what the tools know of
it, and its place in the document.
*/
type apiOperation struct {
	operation mcp.Operation
	raw       map[string]any
}

/*
operations lists the document's operations, by operation id.
*/
func (d apiDocument) operations(doc map[string]any) []apiOperation {
	var out []apiOperation

	for path, item := range asMap(doc["paths"]) {
		for method, raw := range asMap(item) {
			op := asMap(raw)
			id, _ := op["operationId"].(string)

			if id == "" {
				continue
			}

			out = append(out, apiOperation{
				operation: mcp.Operation{ID: id, Method: strings.ToUpper(method), Path: path}, raw: op,
			})
		}
	}

	slices.SortFunc(out, func(a, b apiOperation) int { return strings.Compare(a.operation.ID, b.operation.ID) })

	return out
}

/*
describe writes one operation's words: what it is for, then each
parameter and each field of its body.
*/
func (d apiDocument) describe(op apiOperation) {
	entry := d.docs[op.operation.ID]
	tool := strings.TrimPrefix(toolName(op.operation, entry), toolPrefix)
	b := built{op: op.operation, doc: entry, written: d.written[tool]}

	if entry.Title != "" {
		op.raw["summary"] = entry.Title
		op.raw["description"] = d.plain(purpose(entry))
	}

	for _, raw := range asList(op.raw["parameters"]) {
		param := asMap(raw)
		name, _ := param["name"].(string)

		schema := maps.Clone(asMap(param["schema"]))
		if text, _ := param["description"].(string); text != "" && schema != nil {
			schema["description"] = text
		}

		param["description"] = d.plain(b.argumentText(name, schema))
	}

	content := asMap(asMap(op.raw["requestBody"])["content"])
	body := asMap(asMap(content["application/json"])["schema"])
	d.claim(b, d.resolve(body), fieldPlace{owner: ownerOf(body)})
}

/*
fieldPlace is where a field sits: the schema that owns it and its path
from the body of the operation being described.
*/
type fieldPlace struct {
	owner string
	path  string
}

/*
claim is what one operation says a field of a schema takes.
*/
type claim struct {
	operation string
	text      string
}

/*
ownerOf names the schema a reference points at, empty for a schema
written in place.
*/
func ownerOf(schema map[string]any) string {
	ref, _ := schema["$ref"].(string)

	return strings.TrimPrefix(ref, schemaPrefix)
}

/*
claim gathers what an operation says of every field of a schema, down
through the objects and lists inside it. A field is explained by its
path from the body, such as contact.email, and kept by the schema that
owns it, since several operations may share one schema.
*/
func (d apiDocument) claim(b built, schema map[string]any, at fieldPlace) {
	for _, name := range slices.Sorted(maps.Keys(asMap(schema["properties"]))) {
		field := asMap(asMap(schema["properties"])[name])
		if field == nil || name == "$schema" {
			continue
		}

		path := name
		if at.path != "" {
			path = at.path + "." + name
		}

		target := d.resolve(field)
		if _, isRef := field["$ref"]; !isRef {
			key := at.owner + "." + name
			d.fieldOf[key] = field
			d.claims[key] = append(d.claims[key], claim{
				operation: b.op.ID, text: d.fieldText(b, path, target),
			})
		}

		owner := ownerOf(field)
		if owner == "" {
			owner = at.owner + "." + name
		}

		d.claim(b, target, fieldPlace{owner: owner, path: path})
	}

	if items := asMap(schema["items"]); items != nil {
		owner := ownerOf(items)
		if owner == "" {
			owner = at.owner
		}

		d.claim(b, d.resolve(items), fieldPlace{owner: owner, path: at.path})
	}
}

/*
settle writes each field's description: the one written for a field
several operations explain differently, else what the operations say.
*/
func (d apiDocument) settle() {
	shared := sharedTexts()

	for key, claims := range d.claims {
		text := claims[0].text
		if merged, ok := shared[key]; ok {
			text = merged
		}

		if old, _ := d.fieldOf[key]["description"].(string); old == "" && text != "" {
			d.fieldOf[key]["description"] = d.plain(text)
		}
	}
}

/*
schemaFields writes the description of every field of every schema
that still has none: the text written for it by schema and field, else
what a field of that name means in every record.
*/
func (d apiDocument) schemaFields() {
	texts := schemaTexts()

	for name, raw := range d.schemas {
		for field, value := range asMap(asMap(raw)["properties"]) {
			property := asMap(value)
			if !bare(field, property) {
				continue
			}

			text := texts[name][field]
			if text == "" {
				text = commonText(name, field)
			}

			if text != "" {
				property["description"] = d.plain(text)
			}
		}
	}
}

/*
bare reports whether a field of a schema still needs a description: it
is a field of its own, not a reference, and nothing explains it yet.
*/
func bare(field string, property map[string]any) bool {
	if property == nil || field == "$schema" || property["$ref"] != nil {
		return false
	}

	text, _ := property["description"].(string)

	return text == ""
}

/*
commonText explains a field that means the same in every record: the
page a list answers with, and a record's kind, id and times.
*/
func commonText(schema, field string) string {
	switch {
	case strings.HasSuffix(schema, "MetaStruct"):
		return pageTexts()[field]
	case strings.HasPrefix(schema, "Collection") && field == "data":
		return "The items of this page, as a list; empty when nothing matches."
	case strings.HasPrefix(schema, "Collection") && field == "links":
		return "Addresses of related pages by name, such as next; left out when there are none."
	case strings.HasPrefix(schema, "ResourceList") && field == "data":
		return "The items, as a list; empty when there are none."
	case strings.HasPrefix(schema, "Resource") && field == "data":
		return "The record itself, as a JSON object."
	}

	return recordTexts()[field]
}

/*
pageTexts explains the fields that say where a page of a list sits.
*/
func pageTexts() map[string]string {
	return map[string]string{
		"next_cursor": "Send it as cursor to read the next page; null on the last page.",
		"prev_cursor": "Send it as cursor to read the page before; null on the first page.",
		"per_page":    "How many items a page holds at most, such as 25.",
		"total":       "How many items match in all; left out when the list does not count them.",
	}
}

/*
recordTexts explains the fields every record carries.
*/
func recordTexts() map[string]string {
	return map[string]string{
		"object": "The kind of record this is, the same for every record of its kind, such as article.",
		"id": "The id of this record. Send it wherever an endpoint asks for this record; never use its name " +
			"or title in its place.",
		"created_at": "When it was created, as an RFC 3339 time, such as 2026-01-31T09:00:00Z.",
		"updated_at": "When it last changed, as an RFC 3339 time, such as 2026-01-31T09:00:00Z.",
	}
}

/*
fieldText explains one field: an argument of the tool the way the tool
explains it, a field inside one by the text written for its path.
*/
func (d apiDocument) fieldText(b built, at string, schema map[string]any) string {
	if !strings.Contains(at, ".") {
		return b.argumentText(at, schema)
	}

	if text, ok := b.doc.Params[at]; ok {
		return text
	}

	return b.written[at]
}

/*
resolve follows a reference to the schema it names.
*/
func (d apiDocument) resolve(schema map[string]any) map[string]any {
	ref, ok := schema["$ref"].(string)
	if !ok {
		return schema
	}

	return asMap(d.schemas[strings.TrimPrefix(ref, schemaPrefix)])
}

/*
toolMention finds a tool named in a text.
*/
var toolMention = regexp.MustCompile(`\bdimmi_[a-z0-9_]+`)

/*
plain writes a text for the API document: a tool it names becomes the
endpoint the tool stands for.
*/
func (d apiDocument) plain(text string) string {
	return toolMention.ReplaceAllStringFunc(text, func(name string) string {
		if endpoint, ok := d.endpoints[name]; ok {
			return endpoint
		}

		return name
	})
}

/*
Unexplained lists what an API document leaves without a word, each as
where it is: an operation with no summary or description, a parameter,
or a field of a schema.
*/
func Unexplained(doc map[string]any) []string {
	var out []string

	for path, item := range asMap(doc["paths"]) {
		for method, raw := range asMap(item) {
			out = append(out, undescribedOperation(strings.ToUpper(method)+" "+path, asMap(raw))...)
		}
	}

	for name, raw := range asMap(asMap(doc["components"])["schemas"]) {
		for field, value := range asMap(asMap(raw)["properties"]) {
			if bare(field, asMap(value)) {
				out = append(out, name+"."+field)
			}
		}
	}

	slices.Sort(out)

	return out
}

/*
undescribedOperation lists what one operation leaves without a word:
itself, or a parameter.
*/
func undescribedOperation(at string, op map[string]any) []string {
	if _, ok := op["operationId"].(string); !ok {
		return nil
	}

	var out []string
	if op["description"] == nil || op["summary"] == nil {
		out = append(out, at)
	}

	for _, p := range asList(op["parameters"]) {
		if param := asMap(p); param["description"] == nil {
			out = append(out, fmt.Sprintf("%s: %v", at, param["name"]))
		}
	}

	return out
}

/*
endpointMention finds an endpoint named in a text of the API document.
*/
var endpointMention = regexp.MustCompile(`\b(GET|POST|PUT|PATCH|DELETE) /[A-Za-z0-9_{}/-]+`)

/*
toolsByEndpoint lists the tool that stands for each endpoint.
*/
func toolsByEndpoint(ops []mcp.Operation, docs map[string]Doc) map[string]string {
	out := make(map[string]string, len(ops))
	for _, op := range ops {
		out[op.Method+" "+op.Path] = toolName(op, docs[op.ID])
	}

	return out
}

/*
toolWords writes a text of the API document for a tool's reader: an
endpoint it names becomes the tool that stands for it.
*/
func toolWords(text string, tools map[string]string) string {
	return endpointMention.ReplaceAllStringFunc(text, func(endpoint string) string {
		if tool, ok := tools[endpoint]; ok {
			return tool
		}

		return endpoint
	})
}
