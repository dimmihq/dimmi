/*
|--------------------------------------------------------------------------
| Agent Handoff
|--------------------------------------------------------------------------
|
| Dispatched after the agent stops answering a conversation, naming
| why and carrying the summary the teammate reads first. Listeners
| notify the team and place it where humans pick it up.
|
*/

package events

/*
AiHandoffRequested is dispatched when the agent hands a conversation
to a human: reason says why, summary is the one-paragraph brief.
*/
type AiHandoffRequested struct {
	Base
	ConversationRef

	Reason  string
	Summary string
}

/*
EventName names the event.
*/
func (AiHandoffRequested) EventName() string { return "ai.handoff_requested" }
