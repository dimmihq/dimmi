/*
|--------------------------------------------------------------------------
| Tools From Operations
|--------------------------------------------------------------------------
|
| Each API operation becomes a tool named after it, in the words agents
| expect: list, get, create, update, delete. Its risk follows from what
| it does unless the catalog says otherwise: reads are reads, deletes and
| changes to keys, members, roles and settings are dangerous. Its
| description is the catalog's, with the risk said plainly, and the
| arguments the API leaves unexplained are explained here, with where to
| find the id each one needs.
|
*/

package mcpserver

import (
	"maps"
	"slices"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/mcp"
)

/*
toolPrefix starts every tool's name, so tools from dimmi stand apart
from an agent's other servers.
*/
const toolPrefix = "dimmi_"

/*
confirmArgument is the argument that carries the person's approval of a
tool that needs it, for clients that cannot ask the person themselves.
*/
const confirmArgument = "confirmation_token"

/*
verbs renames the API's action names to the verbs() agents expect.
*/
func verbs() map[string]string {
	return map[string]string{
		"index": "list", "show": "get", "store": "create", "update": "update", "destroy": "delete",
		"update-many": "update_many",
	}
}

/*
toolName names the tool of an operation: dimmi_conversations_replies_create
for conversations.replies.store.
*/
func toolName(operationID string) string {
	parts := strings.Split(operationID, ".")
	if verb, ok := verbs()[parts[len(parts)-1]]; ok {
		parts[len(parts)-1] = verb
	}

	return toolPrefix + strings.ReplaceAll(strings.Join(parts, "_"), "-", "_")
}

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
definition builds the tool definition of an operation.
*/
func definition(op mcp.Operation, doc Doc) mcp.ToolDef {
	r := riskOf(op, doc)

	return mcp.ToolDef{
		Name:        toolName(op.ID),
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
	parts := []string{strings.TrimSpace(doc.Use)}
	if doc.Returns != "" {
		parts = append(parts, "Returns "+strings.TrimSpace(doc.Returns))
	}

	return strings.Join(append(parts, r.caution()), " ")
}

/*
inputSchema copies an operation's input schema with every argument
explained, and the approval argument added to tools that need one.
*/
func inputSchema(op mcp.Operation, doc Doc) map[string]any {
	input := maps.Clone(op.Input)
	properties := map[string]any{}

	for name, raw := range asMap(input["properties"]) {
		schema := maps.Clone(asMap(raw))
		if text := argumentText(name, doc); text != "" {
			schema["description"] = text
		}

		properties[name] = schema
	}

	if riskOf(op, doc).confirms() {
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
argumentText explains an argument: the catalog's words, else the API
document's, else what is known about arguments of that name.
*/
func argumentText(name string, doc Doc) string {
	if text, ok := doc.Params[name]; ok {
		return text
	}

	if text, ok := argumentGuide()[name]; ok {
		return text
	}

	if field, ok := strings.CutPrefix(name, "filter["); ok {
		return "Only items whose " + strings.TrimSuffix(field, "]") + " matches. Several values separated by " +
			"commas match any of them."
	}

	return ""
}

/*
asMap reads a JSON object, nil for anything else.
*/
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}
