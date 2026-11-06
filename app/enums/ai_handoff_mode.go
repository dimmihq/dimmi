/*
|--------------------------------------------------------------------------
| AI Handoff Mode
|--------------------------------------------------------------------------
|
| The workspace decides the agent's role in a conversation: it either
| answers and resolves what it can, or it only triages, sorting the
| request and handing it to the team without ever answering.
|
*/

package enums

/*
AIHandoffMode names the agent's role in a conversation.
*/
type AIHandoffMode string

/*
The modes: full answers from the knowledge, triage only sorts the
conversation and hands it to a person.
*/
const (
	AIHandoffModeFull       AIHandoffMode = "full"
	AIHandoffModeTriageOnly AIHandoffMode = "triage_only"
)

/*
AIHandoffModes returns every mode.
*/
func AIHandoffModes() []AIHandoffMode {
	return []AIHandoffMode{AIHandoffModeFull, AIHandoffModeTriageOnly}
}
