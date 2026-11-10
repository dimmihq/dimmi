/*
|--------------------------------------------------------------------------
| Results Agents Can Act On
|--------------------------------------------------------------------------
|
| Every tool answers twice: the API's JSON as structured content for
| programs, and text written for a model. The text says what happened,
| shows the record or the page of records with ids first and empty
| fields left out, says how to read the next page, and ends with what
| the agent usually does next. Text a customer or anyone outside the
| team wrote is wrapped as untrusted, so the agent treats it as data and
| never as instructions. Long answers are cut where a model would stop
| reading anyway, and say so.
|
*/

package mcpserver

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
Limits on how much text one answer holds.
*/
const (
	maxText      = 12000
	maxValue     = 600
	maxListItems = 50
	maxNested    = 10
	maxDepth     = 3
)

/*
labelFields name a record, in the order the first present one is used.
*/
func labelFields() []string {
	return []string{"name", "title", "subject", "email", "label", "slug", "key", "kind", "type"}
}

/*
factFields describe a record in a list line, after its name.
*/
func factFields() []string {
	return []string{
		"number", "status", "state", "priority", "role", "channel", "visibility", "enabled", "published",
		"assignee", "team", "contact", "unread", "created_at", "updated_at", "last_message_at",
	}
}

/*
outsideFields hold text that comes from customers whoever's record it
is on: a conversation's subject is the customer's email subject, and an
excerpt or preview quotes their messages.
*/
func outsideFields() []string {
	return []string{"subject", "excerpt", "preview"}
}

/*
untrustedFields hold text the person may not have written: what
customers, visitors and outside systems sent.
*/
func untrustedFields() []string {
	return []string{"body", "text", "excerpt", "preview", "content", "description", "comment"}
}

/*
source says who wrote a value: the team, or someone outside it whose
text is untrusted.
*/
type source int

/*
The sources of a value.
*/
const (
	fromTeam source = iota
	fromOutside
)

/*
sourceOf is where a record's text comes from.
*/
func sourceOf(record map[string]any) source {
	if untrusting(record) {
		return fromOutside
	}

	return fromTeam
}

/*
Output is a tool's result: the API's JSON and the text the model reads.
*/
type Output struct {
	text string
	data any
}

/*
Text returns the text the model reads.
*/
func (o Output) Text() string {
	return o.text
}

/*
MarshalJSON sends the API's JSON as the structured content.
*/
func (o Output) MarshalJSON() ([]byte, error) {
	if o.data == nil {
		return []byte(`{}`), nil
	}

	if _, ok := o.data.(map[string]any); !ok {
		return json.Marshal(map[string]any{"data": o.data})
	}

	return json.Marshal(o.data)
}

/*
render writes the text of a successful answer.
*/
func render(doc Doc, tool string, a mcp.Response) Output {
	data := a.Decoded()
	var b strings.Builder

	envelope := asMap(data)
	switch inner := envelope["data"].(type) {
	case []any:
		writeList(&b, title(doc, tool), inner, envelope)
	case map[string]any:
		fmt.Fprintf(&b, "%s: done.\n\n", title(doc, tool))
		writeRecord(&b, inner, 0, sourceOf(inner))
	default:
		fmt.Fprintf(&b, "%s: done.\n", title(doc, tool))
		if data != nil {
			writeValue(&b, field{key: "result"}, data, fromTeam)
		}
	}

	writeNext(&b, doc)

	return Output{text: cut(b.String()), data: data}
}

/*
title names what the tool did.
*/
func title(doc Doc, tool string) string {
	if doc.Title != "" {
		return doc.Title
	}

	return tool
}

/*
writeList writes a page of records, one line each, and how to page on.
*/
func writeList(b *strings.Builder, heading string, items []any, envelope map[string]any) {
	meta := asMap(envelope["meta"])

	switch len(items) {
	case 0:
		fmt.Fprintf(b, "%s: nothing found.", heading)
	case 1:
		fmt.Fprintf(b, "%s: 1 item.", heading)
	default:
		fmt.Fprintf(b, "%s: %d items.", heading, len(items))
	}

	if total, ok := meta["total"].(float64); ok {
		fmt.Fprintf(b, " %d in all.", int(total))
	}

	b.WriteString("\n\n")

	for i, item := range items {
		if i == maxListItems {
			fmt.Fprintf(b, "[%d more not shown]\n", len(items)-maxListItems)

			break
		}

		b.WriteString("- " + listLine(asMap(item), item) + "\n")
	}

	if next, ok := meta["next_cursor"].(string); ok && next != "" {
		fmt.Fprintf(b, "\nMore: call the same tool again with cursor=%q for the next page.\n", next)
	}
}

/*
listLine describes one record of a page: its line, who wrote it when it
was written by someone, and what it says, wrapped as untrusted when it
came from outside the team.
*/
func listLine(record map[string]any, raw any) string {
	text := line(record, raw)
	if record == nil {
		return text
	}

	if who := label(asMap(record["author"])); who != "" {
		text += ", by " + who
	}

	for _, field := range untrustedFields() {
		said, ok := record[field].(string)
		if !ok || strings.TrimSpace(said) == "" {
			continue
		}

		if untrusting(record) {
			return text + "\n  " + envelope(clipLong(said))
		}

		return text + "\n  " + clipLong(said)
	}

	return text
}

/*
line describes one record in a list: its name, its id and its facts.
*/
func line(record map[string]any, raw any) string {
	if record == nil {
		return clip(fmt.Sprint(raw))
	}

	var parts []string
	if name := label(record); name != "" {
		parts = append(parts, name)
	}

	if id, ok := record["id"].(string); ok {
		parts = append(parts, "("+id+")")
	}

	var facts []string
	for _, field := range factFields() {
		if value, ok := fact(record[field]); ok {
			facts = append(facts, field+": "+value)
		}
	}

	if len(facts) > 0 {
		parts = append(parts, "- "+strings.Join(facts, ", "))
	}

	if len(parts) == 0 {
		return clip(compact(record))
	}

	return strings.Join(parts, " ")
}

/*
label returns the first naming field a record has.
*/
func label(record map[string]any) string {
	for _, field := range labelFields() {
		s, ok := record[field].(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}

		if slices.Contains(outsideFields(), field) {
			return envelope(clip(s))
		}

		return clip(s)
	}

	return ""
}

/*
fact writes a short value for a list line, false for one not worth
showing.
*/
func fact(v any) (string, bool) {
	switch value := v.(type) {
	case string:
		return clip(value), value != ""
	case float64:
		return fmt.Sprint(value), true
	case bool:
		return fmt.Sprint(value), value
	case map[string]any:
		if name := label(value); name != "" {
			return name, true
		}

		id, ok := value["id"].(string)

		return id, ok
	default:
		return "", false
	}
}

/*
writeRecord writes a record's fields, id first, empty ones left out.
*/
func writeRecord(b *strings.Builder, record map[string]any, depth int, from source) {
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}

	slices.SortFunc(keys, func(a, c string) int {
		return strings.Compare(order(a), order(c))
	})

	for _, key := range keys {
		writeValue(b, field{key: key, depth: depth}, record[key], fieldSource(from, key))
	}
}

/*
order puts a record's id and name first and its other fields in order.
*/
func order(key string) string {
	switch key {
	case "id":
		return "0"
	case "object", "number":
		return "1" + key
	}

	if slices.Contains(labelFields(), key) {
		return "2" + key
	}

	return "3" + key
}

/*
field is where a value is written: its key and how deep it is nested.
*/
type field struct {
	key   string
	depth int
}

/*
fieldSource is where a field's text comes from: outside the team only
for the text fields of a record written from outside.
*/
func fieldSource(from source, key string) source {
	if slices.Contains(outsideFields(), key) {
		return fromOutside
	}

	if from == fromOutside && slices.Contains(untrustedFields(), key) {
		return fromOutside
	}

	return fromTeam
}

/*
writeValue writes one field, nested records indented below it.
*/
func writeValue(b *strings.Builder, f field, v any, from source) {
	indent := strings.Repeat("  ", f.depth)

	switch value := v.(type) {
	case nil:
		return
	case string:
		writeText(b, indent+f.key, value, from)
	case map[string]any:
		if len(value) == 0 {
			return
		}

		if f.depth >= maxDepth {
			fmt.Fprintf(b, "%s%s: %s\n", indent, f.key, clip(compact(value)))

			return
		}

		fmt.Fprintf(b, "%s%s:\n", indent, f.key)

		nested := from
		if sourceOf(value) == fromOutside {
			nested = fromOutside
		}

		writeRecord(b, value, f.depth+1, nested)
	case []any:
		writeItems(b, f, value, from)
	default:
		fmt.Fprintf(b, "%s%s: %v\n", indent, f.key, value)
	}
}

/*
writeText writes a text field after its label, wrapped as untrusted
when it came from outside the team.
*/
func writeText(b *strings.Builder, label, value string, from source) {
	if value == "" {
		return
	}

	if from == fromOutside {
		fmt.Fprintf(b, "%s: %s\n", label, envelope(clipLong(value)))

		return
	}

	fmt.Fprintf(b, "%s: %s\n", label, clipLong(value))
}

/*
writeItems writes a list field: short values on one line, records one
line each.
*/
func writeItems(b *strings.Builder, f field, items []any, from source) {
	if len(items) == 0 {
		return
	}

	indent := strings.Repeat("  ", f.depth)
	if _, records := items[0].(map[string]any); !records {
		values := make([]string, 0, len(items))
		for _, item := range items {
			values = append(values, fmt.Sprint(item))
		}

		fmt.Fprintf(b, "%s%s: %s\n", indent, f.key, clip(strings.Join(values, ", ")))

		return
	}

	fmt.Fprintf(b, "%s%s (%d):\n", indent, f.key, len(items))

	for i, item := range items {
		if i == maxNested {
			fmt.Fprintf(b, "%s  [%d more]\n", indent, len(items)-maxNested)

			break
		}

		record := asMap(item)
		if from == fromOutside || sourceOf(record) == fromOutside {
			fmt.Fprintf(b, "%s  - %s\n", indent, untrustedLine(record))

			continue
		}

		fmt.Fprintf(b, "%s  - %s\n", indent, line(record, item))
	}
}

/*
untrustedLine describes a record whose text came from outside the team,
its text wrapped.
*/
func untrustedLine(record map[string]any) string {
	head := line(record, record)
	for _, field := range untrustedFields() {
		if text, ok := record[field].(string); ok && text != "" {
			return head + " " + envelope(clipLong(text))
		}
	}

	return head
}

/*
untrusting reports whether a record was written by someone outside the
team: a customer, a visitor, a contact or an outside system.
*/
func untrusting(record map[string]any) bool {
	for _, key := range []string{"author_type", "sender_type", "from_type"} {
		if who, ok := record[key].(string); ok && who != "member" && who != "teammate" && who != "" {
			return true
		}
	}

	author := asMap(record["author"])
	if who, ok := author["type"].(string); ok {
		return who != "member" && who != "teammate"
	}

	return false
}

/*
envelope wraps text from outside the team so the model reads it as
data. Tags inside the text are defused so it cannot close the wrapper.
*/
func envelope(text string) string {
	safe := strings.NewReplacer("<untrusted", "&lt;untrusted", "</untrusted", "&lt;/untrusted").Replace(text)

	return "<untrusted>" + safe + "</untrusted>"
}

/*
writeNext ends an answer with what the agent usually does next.
*/
func writeNext(b *strings.Builder, doc Doc) {
	if len(doc.Next) == 0 {
		return
	}

	b.WriteString("\nNext:\n")

	for _, next := range doc.Next {
		b.WriteString("- " + next + "\n")
	}
}

/*
compact writes a value as one line of JSON.
*/
func compact(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}

	return string(data)
}

/*
clip shortens a value shown on one line.
*/
func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > 120 {
		return string(runes[:119]) + "…"
	}

	return s
}

/*
clipLong shortens a long text field, saying how to read all of it.
*/
func clipLong(s string) string {
	runes := []rune(s)
	if len(runes) <= maxValue {
		return s
	}

	return string(runes[:maxValue]) + fmt.Sprintf(" […%d more characters; the structured content has all of it]",
		len(runes)-maxValue)
}

/*
cut keeps an answer within what a model reads well.
*/
func cut(s string) string {
	runes := []rune(s)
	if len(runes) <= maxText {
		return s
	}

	return string(runes[:maxText]) + "\n[The answer was cut here; the structured content has all of it. " +
		"Ask for fewer items with limit or narrow the request.]"
}
