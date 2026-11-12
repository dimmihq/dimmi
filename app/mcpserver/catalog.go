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
| fails the catalog test, so no tool ever ships undescribed.
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
leaves unexplained, by name. Hint is shown with a failure of this tool,
to steer the agent to the fix it most often needs.
*/
type Doc struct {
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
		outreachDocs(), sourceDocs(), integrationDocs(), webhookDocs(), mergeDocs(), ticketDocs(), incidentDocs(),
		slaDocs(), ticketIssueDocs(), ticketReportDocs(),
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
		"ai.chatgpt.connect.store":  "Connecting a ChatGPT plan needs the person to sign in to OpenAI in a browser.",
		"ai.chatgpt.callback.store": "Finishes the ChatGPT browser sign-in; only the browser calls it.",
	}
}
