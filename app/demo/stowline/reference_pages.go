/*
|--------------------------------------------------------------------------
| API Reference: Pages
|--------------------------------------------------------------------------
|
| One page per operation and one long overview per object, the way a
| real API reference is laid out: what the operation does, its
| parameters, the whole object it returns, a request in every official
| library, a full response, the errors it can return, how it behaves
| under retries and limits, and when it changed. Each section is one
| heading, so a search result carries a whole section, as it would in a
| real help center.
|
*/

package stowline

import (
	"encoding/json"
	"fmt"
	"strings"
)

/*
operation is one operation of a resource, worked out from its verb.
*/
type operation struct {
	verb, method, path, title, about string
}

/*
operations lists the resource's operations, the usual five first.
*/
func operations(r Resource) []operation {
	var out []operation
	for _, v := range r.Ops {
		switch v {
		case "list":
			out = append(out, operation{
				v, "GET", r.Path, "Listing " + r.Plural,
				"Returns your " + r.Plural + ", newest first, a page at a time. Filter by creation time and by the fields marked filterable below.",
			})
		case "create":
			out = append(out, operation{
				v, "POST", r.Path, "Creating a " + r.Name,
				"Creates a " + r.Name + ". " + r.About,
			})
		case "retrieve":
			out = append(out, operation{
				v, "GET", r.Path + "/{id}", "Retrieving a " + r.Name,
				"Returns one " + r.Name + " by its id, with every field.",
			})
		case "update":
			out = append(out, operation{
				v, "PATCH", r.Path + "/{id}", "Updating a " + r.Name,
				"Changes the fields you send and leaves the rest as they are. Fields that cannot change after creation are refused with FIELD_IMMUTABLE.",
			})
		case "delete":
			out = append(out, operation{
				v, "DELETE", r.Path + "/{id}", "Deleting a " + r.Name,
				"Deletes the " + r.Name + ". Deleting is permanent; objects that others still point at are refused with IN_USE.",
			})
		}
	}
	for _, s := range r.Specials {
		out = append(out, operation{s.Verb, s.Method, r.Path + s.Suffix, s.Title, s.About})
	}
	return out
}

/*
referenceArticles are the API reference: an overview per object and a
page per operation.
*/
func referenceArticles() []Article {
	var out []Article
	for _, r := range Resources() {
		out = append(out, resourceOverview(r))
		for _, op := range operations(r) {
			out = append(out, operationPage(r, op))
		}
	}
	return out
}

/*
exampleObject renders the resource's example as JSON, every field.
*/
func exampleObject(r Resource, only func(Field) bool) string {
	var b strings.Builder
	b.WriteString("{\n")
	first := true
	for _, fl := range r.Fields {
		if only != nil && !only(fl) {
			continue
		}
		if !first {
			b.WriteString(",\n")
		}
		first = false
		fmt.Fprintf(&b, "  %q: %s", fl.Name, fl.Example)
	}
	b.WriteString("\n}")
	var v any
	if json.Unmarshal([]byte(b.String()), &v) != nil {
		return b.String()
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	return string(out)
}

/*
fieldTable is the object's fields as a table.
*/
func fieldTable(r Resource, only func(Field) bool) string {
	rows := make([][]string, 0, len(r.Fields))
	for _, fl := range r.Fields {
		if only != nil && !only(fl) {
			continue
		}
		set := "read only"
		if fl.Writable {
			set = "settable"
		}
		rows = append(rows, []string{"`" + fl.Name + "`", fl.Type, set, fl.About})
	}
	return table([]string{"Field", "Type", "Set by you", "Description"}, rows)
}

func operationPage(r Resource, op operation) Article {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", op.about)
	b.WriteString("## Endpoint\n\n")
	b.WriteString(code("http", op.method+" https://api.stowline.test"+op.path))
	fmt.Fprintf(&b, "\nAuthenticate with a key carrying the `%s:%s` scope. Available since API version %s. "+
		"Requests and responses are JSON encoded as UTF-8; amounts are decimal strings.\n\n", r.Scope,
		map[bool]string{true: "read", false: "write"}[op.method == "GET"], r.Since)
	b.WriteString(paramsSection(r, op))
	fmt.Fprintf(&b, "## The %s object\n\n%s\n\n", r.Name, r.About)
	b.WriteString(fieldTable(r, nil))
	b.WriteString("\n")
	b.WriteString(requestSection(r, op))
	b.WriteString("## Example response\n\n")
	b.WriteString(code("json", responseFor(r, op)))
	b.WriteString("\n")
	b.WriteString(errorsSection(r, op))
	b.WriteString(behaviorSection(r, op))
	b.WriteString(historySection(r, op))
	return Article{Collection: "api", Title: op.title, Description: firstSentence(op.about), Body: b.String()}
}

/*
paramsSection lists what the request carries.
*/
func paramsSection(r Resource, op operation) string {
	var b strings.Builder
	b.WriteString("## Parameters\n\n")
	if strings.Contains(op.path, "{id}") {
		fmt.Fprintf(&b, "The path carries the %s's `id`.\n\n", r.Name)
	}
	switch op.verb {
	case "list":
		b.WriteString(table([]string{"Parameter", "Type", "Description"}, [][]string{
			{"`limit`", "integer", "How many to return, 1 to 100, default 20."},
			{"`cursor`", "string", "The next_cursor of the previous page."},
			{"`created_after`, `created_before`", "timestamp", "Only objects created in the range, RFC 3339."},
			{"`expand`", "array", "Related objects to include in full instead of their ids."},
			{"`mode`", "enum", "live or test; defaults to the key's mode."},
		}))
	case "create", "update":
		b.WriteString("Send the settable fields in the body. ")
		if op.verb == "create" {
			b.WriteString("Fields you leave out take their defaults.\n\n")
		} else {
			b.WriteString("Fields you leave out keep their values; send null to clear a field that may be empty.\n\n")
		}
		b.WriteString(fieldTable(r, func(fl Field) bool { return fl.Writable }))
	default:
		b.WriteString("This operation takes no body unless noted in its description. ")
		b.WriteString("Send an `Idempotency-Key` header with every POST; repeating a request with the same key within 24 hours returns the first result.\n")
	}
	b.WriteString("\n")
	return b.String()
}

/*
requestSection shows the request with curl and each official library.
*/
func requestSection(r Resource, op operation) string {
	path := strings.ReplaceAll(op.path, "{id}", strings.Trim(r.Fields[0].Example, `"`))
	body := ""
	if op.verb == "create" || op.verb == "update" {
		body = exampleObject(r, func(fl Field) bool { return fl.Writable })
	}
	var b strings.Builder
	b.WriteString("## Example request\n\n")
	curl := fmt.Sprintf("curl -X %s https://api.stowline.test%s \\\n  -H \"Authorization: Bearer $STOWLINE_API_KEY\"",
		op.method, path)
	if op.method != "GET" {
		curl += " \\\n  -H \"Idempotency-Key: 4b1d-" + r.Key + "-" + op.verb + "\""
	}
	if body != "" {
		curl += " \\\n  -H \"Content-Type: application/json\" \\\n  -d '" + body + "'"
	}
	b.WriteString(code("bash", curl))
	method := camel(strings.ReplaceAll(r.Plural, " ", "_")) + "." + map[string]string{
		"retrieve": "retrieve",
		"list":     "list", "create": "create", "update": "update", "delete": "del",
	}[op.verb]
	if strings.HasSuffix(method, ".") {
		method += op.verb
	}
	fmt.Fprintf(&b, "\n**Node.js**\n\n%s\n**Python**\n\n%s\n**Ruby**\n\n%s\n**PHP**\n\n%s\n",
		code("javascript", "const result = await stowline."+method+"("+argsFor(op, "{ ... }")+");"),
		code("python", "result = client."+snake(method)+"("+argsFor(op, "**params")+")"),
		code("ruby", "result = client."+snake(method)+"("+argsFor(op, "params")+")"),
		code("php", "$result = $stowline->"+strings.ReplaceAll(method, ".", "->")+"("+argsFor(op, "$params")+");"))
	return b.String()
}

func argsFor(op operation, params string) string {
	switch {
	case strings.Contains(op.path, "{id}") && (op.verb == "update" || op.method == "POST"):
		return `"id", ` + params
	case strings.Contains(op.path, "{id}"):
		return `"id"`
	case op.verb == "list":
		return "{ limit: 20 }"
	}
	return params
}

/*
responseFor is the example response of the operation.
*/
func responseFor(r Resource, op operation) string {
	switch op.verb {
	case "list":
		return "{\n  \"data\": [\n" + indent(exampleObject(r, nil), "    ") +
			"\n  ],\n  \"next_cursor\": \"c_eyJpZCI6IjAxSkM4WjRNMlEifQ\"\n}"
	case "delete":
		return fmt.Sprintf("{\n  \"id\": %s,\n  \"deleted\": true\n}", r.Fields[0].Example)
	}
	return exampleObject(r, nil)
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

/*
errorsSection lists the errors the operation can return.
*/
func errorsSection(r Resource, op operation) string {
	rows := [][]string{
		{"`UNAUTHORIZED`", "401", "The key is missing, mistyped or revoked."},
		{"`FORBIDDEN`", "403", "The key lacks the " + r.Scope + " scope this operation needs; the message names it."},
		{"`RATE_LIMITED`", "429", "Too many requests; wait for Retry-After seconds."},
	}
	if strings.Contains(op.path, "{id}") {
		rows = append(rows, []string{"`NOT_FOUND`", "404", "No " + r.Name + " with that id in this mode. Test and live objects never see each other."})
	}
	if op.verb == "create" || op.verb == "update" {
		rows = append(rows, []string{"`VALIDATION_FAILED`", "422", "A field is missing or invalid; `error.field` names it with a dotted path such as to_address.postal_code."})
	}
	for _, e := range ErrorCodes() {
		if strings.Contains(strings.ToLower(e.Meaning+e.Title), strings.Fields(strings.ToLower(r.Name))[0]) {
			rows = append(rows, []string{"`" + e.Code + "`", fmt.Sprint(e.Status), e.Meaning})
		}
	}
	return "## Errors\n\n" + table([]string{"Code", "Status", "When"}, rows) +
		"\nEvery error body has the same shape: `code`, `message`, `field` when one is to blame, and `request_id`. " +
		"Branch on the code; messages are written for people and change.\n\n"
}

/*
behaviorSection says how the operation behaves under retries, limits
and concurrency.
*/
func behaviorSection(_ Resource, op operation) string {
	var items []string
	switch op.method {
	case "GET":
		items = append(items, "Reads are served from a replica and can lag writes by up to two seconds. Read your own write by passing the `Stowline-Consistency: strong` header, which counts double against the rate limit.")
	case "POST":
		items = append(items, "Idempotency keys are kept for 24 hours per key and endpoint. A repeated key with a different body is refused with IDEMPOTENCY_MISMATCH rather than applied.")
	case "PATCH":
		items = append(items, "Concurrent updates are applied in the order they arrive; send `If-Match` with the object's `updated_at` to refuse an update when someone changed it since you read it.")
	case "DELETE":
		items = append(items, "Deleting an object that is already gone answers 404, which you can treat as done.")
	}
	items = append(items,
		"The general limit is 100 requests a second per account in live mode and 25 in test mode. Rating has its own limit of 20 a second and address verification 10.",
		"Test mode behaves the same but never reaches a carrier and never charges. Test labels carry the word SAMPLE.",
		"New fields can appear in responses at any time; ignore fields you do not know.")
	return "## Behavior and limits\n\n" + list(items...) + "\n"
}

/*
historySection is when the operation changed, by API version.
*/
func historySection(r Resource, op operation) string {
	rows := [][]string{{r.Since, "The " + r.Name + " object and this operation were introduced."}}
	if op.verb == "list" {
		rows = append(rows, []string{"2025-03-11", "Cursor pagination replaced offsets; `offset` still works on versions before this one."})
	}
	if op.method == "POST" {
		rows = append(rows, []string{"2025-09-02", "Repeating an idempotency key with a different body is refused instead of returning the first result."})
	}
	rows = append(rows, []string{"2026-03-01", "Amounts became decimal strings everywhere; earlier versions return numbers for some fields."})
	return "## Version history\n\n" + table([]string{"Version", "Change"}, rows) +
		"\nYour account is pinned to the version current at your first request. Send `Stowline-Version` to use another.\n"
}

/*
resourceOverview is the long page on one object: every field explained
at length, every operation, and how the object moves through its life.
*/
func resourceOverview(r Resource) Article {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nThis page describes the %s object in full and lists every operation on it. "+
		"Each operation has its own page with examples in every official library.\n\n", r.About, r.Name)
	b.WriteString("## Operations\n\n")
	rows := [][]string{}
	for _, op := range operations(r) {
		rows = append(rows, []string{op.method, "`" + op.path + "`", op.title, firstSentence(op.about)})
	}
	b.WriteString(table([]string{"Method", "Path", "Page", "What it does"}, rows))
	fmt.Fprintf(&b, "\n## Fields\n\n%s\n", fieldTable(r, nil))
	for _, fl := range r.Fields {
		if !strings.Contains(fl.Type, "object") && !strings.Contains(fl.Type, "array") && fl.Type != "enum" {
			continue
		}
		fmt.Fprintf(&b, "## The %s field\n\n%s\n\nExample:\n\n%s\n%s\n", fl.Name, fl.About,
			code("json", fmt.Sprintf("{%q: %s}", fl.Name, fl.Example)), fieldNote(r, fl))
	}
	fmt.Fprintf(&b, "## A complete example\n\n%s\n", code("json", exampleObject(r, nil)))
	for _, op := range operations(r) {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n%s\n", op.title, op.about,
			code("http", op.method+" https://api.stowline.test"+op.path))
		b.WriteString(demote(paramsSection(r, op)))
		b.WriteString(demote(requestSection(r, op)))
		b.WriteString("### Response\n\n" + code("json", responseFor(r, op)) + "\n")
	}
	b.WriteString("## Keeping your copy in sync\n\n" + list(
		"Store the id of every "+r.Name+" you create beside your own record, in the same transaction.",
		"Prefer webhooks to polling: subscribe to the events for "+r.Plural+" and use `occurred_at` to order them.",
		"For a nightly reconciliation, list "+r.Plural+" created since your last run with `created_after` and the largest `limit`.",
		"Never parse error messages; branch on error codes.",
	) + "\n")
	return Article{
		Collection: "api", Title: "The " + r.Name + " object", Description: firstSentence(r.About),
		Body: b.String(),
	}
}

/*
fieldNote adds what support most often explains about a field.
*/
func fieldNote(_ Resource, fl Field) string {
	switch {
	case fl.Type == "enum":
		return "Values can be added in new API versions without notice. Store unknown values as they are and treat them as the closest known state, never as an error."
	case strings.Contains(fl.Type, "array"):
		return "Arrays are returned in full up to 100 items; longer lists are cut and `has_more` is set on the containing object. Ask the dedicated list operation for the rest."
	default:
		return "Nested objects are returned in full. Their own ids, where they have one, can be used with the matching operations."
	}
}

func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i > 0 {
		return s[:i+1]
	}
	return s
}

/*
demote turns a page's sections into subsections, so a document that
gathers several pages keeps one section per page.
*/
func demote(body string) string {
	if strings.HasPrefix(body, "## ") {
		body = "#" + body
	}
	return strings.ReplaceAll(body, "\n## ", "\n### ")
}
