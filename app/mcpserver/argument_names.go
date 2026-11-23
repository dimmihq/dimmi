/*
|--------------------------------------------------------------------------
| Argument Names
|--------------------------------------------------------------------------
|
| The API names a record in a path by what it is, {article}, and a field
| by the shortest word that fits, name or body. A model that reads only
| those names fills them wrong: a title where an id belongs, the wrong
| text in body. So a tool's arguments are named for what they hold: an
| id ends in _id, a filter starts with filter_, and a word too bare to
| read alone carries the thing it belongs to, such as reply_body. The
| API keeps its own names; a call is turned back into them before it is
| made, and an argument sent under another likely name is still read.
|
*/

package mcpserver

import (
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
conversationArgument is the argument that names a conversation in a
path, where the API takes its id or its number.
*/
const conversationArgument = "conversation_id_or_number"

/*
searchArgument is the argument that carries search words.
*/
const searchArgument = "search_text"

/*
bareWords are argument names that say nothing alone, so each is named
after the thing the tool is about.
*/
func bareWords() []string {
	return []string{"name", "title", "body", "description", "subject", "number", "since", "text"}
}

/*
plainPaths are the path arguments that are not ids, or whose record
goes by another word here, with the name each goes by.
*/
func plainPaths() map[string]string {
	return map[string]string{
		"emoji": "emoji", "provider": "integration_provider", "report": "report_name", "scope": "snooze_list_scope",
		"changeset": "suggestion_id", "skill": "skill_name",
	}
}

/*
idExample finds an id written as an example in a description, such as
(conv_...).
*/
var idExample = regexp.MustCompile(`\b[a-z]{2,6}_\.{3}`)

/*
naming is how one tool's arguments are named: the API's name of each,
and the other names an argument is still read under.
*/
type naming struct {
	api     map[string]string
	aliases map[string]string
}

/*
namingOf works out the names of an operation's arguments.
*/
func namingOf(op mcp.Operation, doc Doc) naming {
	n := naming{api: map[string]string{}, aliases: map[string]string{}}
	properties := asMap(op.Input["properties"])
	path := pathArguments(op.Path)

	names := slices.Sorted(maps.Keys(properties))
	for _, name := range names {
		renamed := argumentName(op, doc, name, asMap(properties[name]))
		if _, taken := properties[renamed]; taken && renamed != name {
			renamed = name
		}

		if _, taken := n.api[renamed]; taken {
			renamed = name
		}

		n.api[renamed] = name
	}

	for renamed, name := range n.api {
		for _, alias := range aliasesOf(name, renamed, path) {
			if _, named := n.api[alias]; !named {
				n.aliases[alias] = renamed
			}
		}
	}

	return n
}

/*
argumentName names one argument for the model: the catalog's name when
it gives one, else by what the argument holds.
*/
func argumentName(op mcp.Operation, doc Doc, name string, schema map[string]any) string {
	if renamed, ok := doc.Rename[name]; ok {
		return renamed
	}

	if slices.Contains(pathArguments(op.Path), name) {
		if plain, ok := plainPaths()[name]; ok {
			return plain
		}

		if name == "conversation" {
			return conversationArgument
		}

		return strings.TrimSuffix(name, "_id") + "_id"
	}

	if field, ok := strings.CutPrefix(name, "filter["); ok {
		field = strings.TrimSuffix(field, "]")
		description, _ := schema["description"].(string)

		if idExample.MatchString(description) && !strings.HasSuffix(field, "_id") {
			field += "_id"
		}

		return "filter_" + field
	}

	if name == "q" || name == "query" {
		return searchArgument
	}

	if word := resourceWord(op.ID); word != "" && slices.Contains(bareWords(), name) {
		return word + "_" + name
	}

	return name
}

/*
aliasesOf are the other names an argument is read under: the API's own
name, and what a model that half remembers the name is likely to send.
*/
func aliasesOf(name, renamed string, path []string) []string {
	out := []string{name}

	if field, ok := strings.CutPrefix(renamed, "filter_"); ok {
		out = append(out, field, strings.TrimSuffix(field, "_id"))
	}

	switch renamed {
	case conversationArgument:
		out = append(out, "conversation_id", "conversation_number", "number")
	case searchArgument:
		out = append(out, "q", "query", "search")
	}

	if len(path) > 0 && path[len(path)-1] == name {
		out = append(out, "id")
	}

	return out
}

/*
names lists the arguments by the names the model sees.
*/
func (n naming) names() []string {
	return slices.Sorted(maps.Keys(n.api))
}

/*
shown is the name the model sees for one of the API's names, which may
be a field inside an argument, such as contact.email.
*/
func (n naming) shown(name string) string {
	head, rest, nested := strings.Cut(name, ".")

	for renamed, api := range n.api {
		if api != head {
			continue
		}

		if nested {
			return renamed + "." + rest
		}

		return renamed
	}

	return name
}

/*
toAPI turns a call's arguments back into the names the API expects.
*/
func (n naming) toAPI(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))

	for name, value := range args {
		if api, ok := n.api[name]; ok {
			out[api] = value

			continue
		}

		out[name] = value
	}

	return out
}
