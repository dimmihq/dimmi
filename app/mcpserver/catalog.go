/*
|--------------------------------------------------------------------------
| Tool Catalog
|--------------------------------------------------------------------------
|
| Every API operation an agent can call is a tool, and every tool has a
| written entry here: what it is for, when to reach for it, what comes
| back, what to do next and how risky it is. The tools themselves are
| generated from the API document, so they never drift from the API; the
| catalog is what makes them teach. A new API operation without an entry
| fails the catalog test, so no tool ever ships undescribed. The
| operations that are not tools have entries too: the API document is
| described from this catalog, and it explains every operation.
|
*/

package mcpserver

import "maps"

/*
Risk is how carefully an agent must treat a tool.

  - RiskRead reads workspace data.
  - RiskWrite changes workspace data only the team sees.
  - RiskSensitive reads data that is private or bulky, such as personal
    data exports, audit trails or secrets; the person approves first.
  - RiskReach is seen outside the team: a message to a customer, a
    published article or entry, a change to what customers see; the
    person approves first.
  - RiskDanger is hard to undo or changes how the workspace works:
    deleting, members, roles, security, keys, billing and settings; the
    person approves first.
*/
type Risk int

/*
The risks, from safest to most careful.
*/
const (
	RiskRead Risk = iota
	RiskWrite
	RiskSensitive
	RiskReach
	RiskDanger
)

/*
Doc is a tool's catalog entry. Title names it for people. Use says what
it does and when to reach for it, in a sentence or three. Returns says
what comes back. Next lists what an agent usually does after it, by
tool name and why. Risk overrides the risk worked out from the method
and abilities when set. Params explains arguments the API document
leaves unexplained, by the API's name of each. Hint is shown with a
failure of this tool, to steer the agent to the fix it most often
needs. Name is the tool's name, without the dimmi_ prefix, when the one
worked out from the operation does not say what the tool does.
*/
type Doc struct {
	Name    string
	Title   string
	Use     string
	Returns string
	Next    []string
	Risk    *Risk
	Params  map[string]string
	Hint    string
}

/*
catalog is every entry, by operation id.
*/
func catalog() map[string]Doc {
	out := map[string]Doc{}
	for _, area := range []map[string]Doc{
		inboxDocs(), peopleDocs(), knowledgeDocs(), automationDocs(), workspaceDocs(), settingsDocs(),
		outreachDocs(),
		sourceDocs(),
		integrationDocs(),
		webhookDocs(),
		mergeDocs(),
		ticketDocs(),
		trackerDocs(),
		slaDocs(), ticketIssueDocs(), ticketReportDocs(), channelDocs(), noteDocs(), unexposedDocs(),
	} {
		maps.Copy(out, area)
	}

	return out
}

/*
excluded names API operations that are not tools, and why. They need a
person at a browser, such as finishing a sign-in with a provider, so an
agent could never complete them.
*/
func excluded() map[string]string {
	return map[string]string{
		"ai.chatgpt.connect.store":               "Connecting ChatGPT needs a person to sign in to OpenAI.",
		"ai.chatgpt.callback.store":              "Finishes the ChatGPT browser sign-in; only the browser calls it.",
		"channels.whatsapp.signup-config.index":  "What Meta's signup popup opens with; only the browser uses it.",
		"channels.whatsapp.signups.store":        "Finishes Meta's signup in a browser; only the browser calls it.",
		"channels.facebook.signup-config.index":  "What Meta's sign-in popup opens with; only the browser uses it.",
		"channels.facebook.signups.store":        "Finishes Meta's sign-in in a browser; only the browser calls it.",
		"channels.instagram.signup-config.index": "What Meta's sign-in popup opens with; only the browser uses it.",
		"channels.instagram.signups.store":       "Finishes Meta's sign-in in a browser; only the browser calls it.",
		"migrations.records.store": "Records carry customer data, which never passes through an AI model; " +
			"the dimmi CLI sends them.",
	}
}
