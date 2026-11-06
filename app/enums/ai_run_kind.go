/*
|--------------------------------------------------------------------------
| AI Run Kind
|--------------------------------------------------------------------------
|
| Every time the agent's machinery runs, a run row records why: a real
| customer message, a playground question, or a tiny probe. The kinds
| let costs, retention and audit each treat a run correctly.
|
*/

package enums

/*
AIRunKind names what started one run of the agent's machinery.
*/
type AIRunKind string

/*
The run kinds.
*/
const (
	AIRunKindConversation AIRunKind = "conversation"
	AIRunKindPlayground   AIRunKind = "playground"
	AIRunKindProbe        AIRunKind = "probe"
	AIRunKindHelpCenter   AIRunKind = "help_center"
	AIRunKindTask         AIRunKind = "task"
)
