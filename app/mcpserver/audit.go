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
do not exist, as "where: name". Tools lists every tool name.
*/
type Report struct {
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
	return len(r.Undocumented)+len(r.Stale)+len(r.Incomplete)+len(r.Unknown) == 0
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

	var r Report

	for _, op := range ops {
		known[toolName(op.ID)] = true
		entry, ok := docs[op.ID]

		if !ok {
			r.Undocumented = append(r.Undocumented, op.ID)

			continue
		}

		if strings.TrimSpace(entry.Title) == "" || strings.TrimSpace(entry.Use) == "" {
			r.Incomplete = append(r.Incomplete, op.ID)
		}
	}

	for id := range docs {
		if !known[toolName(id)] {
			r.Stale = append(r.Stale, id)
		}
	}

	r.Unknown = unknownReferences(docs, known)

	for name := range known {
		r.Tools = append(r.Tools, name)
	}

	for _, list := range [][]string{r.Undocumented, r.Stale, r.Incomplete, r.Unknown, r.Tools} {
		slices.Sort(list)
	}

	return r, nil
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
