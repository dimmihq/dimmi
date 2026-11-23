/*
|--------------------------------------------------------------------------
| Catalog Audit
|--------------------------------------------------------------------------
|
| The tools follow the API by construction, but the catalog is written by
| hand, so it is checked: every operation an agent can call has an entry,
| every entry belongs to an operation that exists, every entry is
| complete, and every tool a text names is a real tool. The test suite
| runs the audit, so the MCP server cannot fall behind the API.
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
	"github.com/dimmihq/dimmi/framework/routing"
)

/*
Report is what the audit found. Undocumented lists callable operations
without an entry, Stale entries without an operation, Incomplete
entries missing a title or a use, and Unknown references to tools that
do not exist, as "where: name". Duplicate lists names two operations
would share, Unclear arguments a model could not read from their
name alone, and Unwritten arguments nobody explained, left with only
what their name and type say, both as "tool: argument". Tools lists
every tool name.
*/
type Report struct {
	Duplicate    []string
	Unclear      []string
	Unwritten    []string
	Undocumented []string
	Stale        []string
	Incomplete   []string
	Unknown      []string
	Tools        []string
}

/*
Clean reports whether the audit found nothing wrong.
*/
func (r Report) Clean() bool {
	return len(r.Undocumented)+len(r.Stale)+len(r.Incomplete)+len(r.Unknown)+len(r.Duplicate)+len(r.Unclear)+
		len(r.Unwritten) == 0
}

/*
toolReference finds tool names in text.
*/
var toolReference = regexp.MustCompile(`\bdimmi_[a-z0-9_]+`)

/*
Audit checks the catalog against the router's API.
*/
func Audit(router *routing.Router) (Report, error) {
	doc, err := router.OpenAPI(apiPrefix)
	if err != nil {
		return Report{}, fmt.Errorf("read api document: %w", err)
	}

	ops, err := mcp.Operations(doc, callable(router))
	if err != nil {
		return Report{}, fmt.Errorf("read api operations: %w", err)
	}

	docs := catalog()
	known := map[string]bool{toolPrefix + "guide": true}
	live := map[string]bool{}

	var r Report

	for _, op := range ops {
		live[op.ID] = true
		check(&r, op, docs, known)
	}

	for id := range docs {
		if !live[id] {
			r.Stale = append(r.Stale, id)
		}
	}

	r.Unknown = unknownReferences(docs, known)
	r.Tools = slices.Collect(maps.Keys(known))
	r.sort()

	return r, nil
}

/*
check audits one operation: its entry, its name and its arguments.
*/
func check(r *Report, op mcp.Operation, docs map[string]Doc, known map[string]bool) {
	entry, ok := docs[op.ID]
	name := toolName(op, entry)

	if known[name] {
		r.Duplicate = append(r.Duplicate, name)
	}

	known[name] = true
	b := build(op, entry)
	r.Unclear = append(r.Unclear, unclear(b)...)
	r.Unwritten = append(r.Unwritten, unwritten(b)...)

	switch {
	case !ok:
		r.Undocumented = append(r.Undocumented, op.ID)
	case strings.TrimSpace(entry.Title) == "" || strings.TrimSpace(entry.Use) == "":
		r.Incomplete = append(r.Incomplete, op.ID)
	}
}

/*
sort orders every list of the report.
*/
func (r Report) sort() {
	for _, list := range [][]string{
		r.Undocumented, r.Stale, r.Incomplete, r.Unknown, r.Duplicate, r.Unclear, r.Unwritten, r.Tools,
	} {
		slices.Sort(list)
	}
}

/*
unclear lists the arguments of a tool a model could not read from their
name alone: a bare word, a search called q, a filter in brackets, a
record named in the path without saying it is an id, or an argument
nothing explains.
*/
func unclear(b built) []string {
	var out []string

	path := pathArguments(b.op.Path)
	for name, api := range b.names.api {
		_, plain := plainPaths()[api]
		schema := asMap(asMap(b.schema["properties"])[name])
		text, _ := schema["description"].(string)

		switch {
		case slices.Contains(bareWords(), name), name == "q", strings.Contains(name, "["), text == "":
			out = append(out, b.def.Name+": "+name)
		case slices.Contains(path, api) && !plain && !strings.Contains(name, "_id"):
			out = append(out, b.def.Name+": "+name)
		}
	}

	return out
}

/*
unwritten lists the arguments of a tool nobody explained: those whose
description is only what their name and type say.
*/
func unwritten(b built) []string {
	var out []string

	for name := range b.names.api {
		schema := asMap(asMap(b.schema["properties"])[name])
		if text, _ := schema["description"].(string); text == plainText(name, schema) {
			out = append(out, b.def.Name+": "+name)
		}
	}

	return out
}

/*
unknownReferences lists tool names written in the catalog, the argument
guide and the instructions that are not tools.
*/
func unknownReferences(docs map[string]Doc, known map[string]bool) []string {
	texts := map[string]string{"instructions": instructions}
	for topic, text := range guideTopics() {
		texts["guide "+topic] = text
	}

	for name, text := range argumentGuide() {
		texts["argument "+name] = text
	}

	for id, entry := range docs {
		texts[id] = strings.Join(append([]string{entry.Use, entry.Returns, entry.Hint}, entry.Next...), " ")
		for name, text := range entry.Params {
			texts[id+" "+name] += " " + text
		}
	}

	var out []string

	for where, text := range texts {
		for _, name := range toolReference.FindAllString(text, -1) {
			if !known[name] {
				out = append(out, where+": "+name)
			}
		}
	}

	return out
}
