/*
|--------------------------------------------------------------------------
| Argument Names
|--------------------------------------------------------------------------
|
| A tool's arguments carry the very names the API gives them, in the
| path, the query and the body, so what a model reads from one tool it
| can send to another and what the API reference says holds for the
| tools too. Clarity comes from each argument's description, not from a
| second vocabulary. A model that half remembers a name is still read:
| article_id for article, search_text for q, filter_status for
| filter[status], id for the record a path ends with.
|
*/

package mcpserver

import (
	"maps"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
customFilter starts the filters by a custom attribute, such as
filter[custom.plan], which no schema can list since the keys are the
workspace's own.
*/
const customFilter = "filter[custom."

/*
bareWords are argument names that say nothing alone, so each is also
read under the name of the thing the tool is about: reply_body for
body.
*/
func bareWords() []string {
	return []string{"name", "title", "body", "description", "subject", "number", "since", "text"}
}

/*
naming is the arguments of one tool by name, and the other names each
is still read under.
*/
type naming struct {
	api     map[string]string
	aliases map[string]string
}

/*
namingOf lists an operation's arguments and the other names they are
read under.
*/
func namingOf(op mcp.Operation) naming {
	n := naming{api: map[string]string{}, aliases: map[string]string{}}
	path := pathArguments(op.Path)

	for name := range asMap(op.Input["properties"]) {
		n.api[name] = name
	}

	for name := range n.api {
		for _, alias := range aliasesOf(op, name, path) {
			if _, named := n.api[alias]; !named {
				n.aliases[alias] = name
			}
		}
	}

	return n
}

/*
aliasesOf are the other names an argument is read under: what a model
that half remembers the name is likely to send.
*/
func aliasesOf(op mcp.Operation, name string, path []string) []string {
	var out []string

	if slices.Contains(path, name) {
		out = append(out, strings.TrimSuffix(name, "_id")+"_id")
		if path[len(path)-1] == name {
			out = append(out, "id")
		}
	}

	if field, ok := strings.CutPrefix(name, "filter["); ok {
		field = strings.TrimSuffix(field, "]")
		out = append(out, field, "filter_"+field, "filter_"+field+"_id")
	}

	switch name {
	case "conversation":
		out = append(out, "conversation_number", "conversation_id_or_number", "number")
	case "q", "query":
		out = append(out, "search_text", "search", "q", "query")
	}

	if word := resourceWord(op.ID); word != "" && slices.Contains(bareWords(), name) {
		out = append(out, word+"_"+name)
	}

	return out
}

/*
names lists the arguments by name.
*/
func (n naming) names() []string {
	return slices.Sorted(maps.Keys(n.api))
}
