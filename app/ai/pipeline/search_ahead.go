/*
|--------------------------------------------------------------------------
| Searching Ahead
|--------------------------------------------------------------------------
|
| Most first messages are a question, and the customer's own sentence
| is the best search there is for it: it says what they mean in the
| words they would look for. So before the agent's first model call the
| knowledge is searched with that sentence, and what it found is put in
| front of the agent as a search already made: the call and its result,
| in the same shape as one the agent makes itself. The agent then
| writes its answer in one model call instead of two, and starts from
| the passages the customer's wording finds, whatever query it would
| have written.
|
| It happens once, on the customer's first message, before anyone has
| replied in the conversation. Later messages lean on what was said
| before them, so the agent searches for those itself. A message too
| short to be a question, such as a greeting, is not searched. A search
| that found nothing is left out, so the agent looks for itself rather
| than read an empty result as the answer.
|
| The search is not one the agent chose to make. It does not count
| against the searches the agent may make in the turn, and a reply
| that uses none of it, such as a question back to the customer, is
| read as it was before: not as an agent that looked and found nothing.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
aheadMinBytes is the shortest first message worth searching with. It
is measured in bytes so it holds across scripts: about fifteen Latin
letters, eight Cyrillic or Arabic ones, five Chinese or Japanese
characters. "Hello, good day" falls under it; "Do you support SSO?"
does not.
*/
const aheadMinBytes = 16

/*
aheadCallID names the search made ahead of the agent among the turn's
tool calls.
*/
const aheadCallID = "search_ahead_1"

/*
aheadNote tells the agent where the result came from and what to do
when it falls short.
*/
const aheadNote = "Searched with the customer's own message before you started. It covers their wording and " +
	"nothing more. Before you reply, list every fact the reply needs: each thing they asked, and each thing the " +
	"team's instructions tell you to bring up or look up at this moment. Any of them not shown in these results, " +
	"search again yourself for it, from another angle. A search costs little; a reply that leaves out something " +
	"the team wanted said costs the sale."

/*
searchAhead searches the knowledge with the customer's first message
and adds the call and its result to the conversation the agent reads.
*/
func (r *Runner) searchAhead(ctx context.Context, st *run) {
	question := r.aheadQuestion(st)
	if question == "" || !r.canSearch(st) {
		return
	}
	args, err := json.Marshal(map[string]string{"question": question})
	if err != nil {
		return
	}
	call := contracts.ToolCall{ID: aheadCallID, Name: toolSearchKnowledge, Arguments: args}

	st.ahead = true
	result, failed := r.callTool(ctx, st, call)
	st.ahead = false
	if failed || st.lastSearch.Kept == 0 {
		return
	}
	st.loop.messages = append(st.loop.messages,
		contracts.ChatMessage{Role: contracts.ChatRoleAssistant, ToolCalls: []contracts.ToolCall{call}},
		contracts.ChatMessage{
			Role: contracts.ChatRoleTool, ToolCallID: call.ID,
			Parts: []contracts.ContentPart{{Text: withNote(result)}},
		})
}

/*
aheadQuestion is the message to search with: the customer's first
message, when nothing was said in the conversation before it and it is
long enough to be a question. Otherwise it is empty. The agent reads
the search as its own call, so the message is masked as every message
it reads is.
*/
func (r *Runner) aheadQuestion(st *run) string {
	if st.proc.active() || len(st.env.History) > 0 {
		return ""
	}
	question := strings.Join(strings.Fields(r.redactFor(st.env, st.env.pendingText())), " ")
	if len(question) < aheadMinBytes {
		return ""
	}
	if runes := []rune(question); len(runes) > aheadMaxRunes {
		question = string(runes[:aheadMaxRunes])
	}
	return question
}

/*
aheadMaxRunes is the most of a long first message that is searched
with.
*/
const aheadMaxRunes = 600

/*
canSearch reports whether the agent is offered the knowledge search in
this turn.
*/
func (r *Runner) canSearch(st *run) bool {
	return slices.ContainsFunc(r.toolSpecs(st), func(t contracts.ToolSpec) bool {
		return t.Name == toolSearchKnowledge
	})
}

/*
withNote adds to a search result the note saying it was made ahead of
the agent.
*/
func withNote(result string) string {
	var body map[string]any
	if json.Unmarshal([]byte(result), &body) != nil {
		return result
	}
	body["note"] = aheadNote
	out, err := json.Marshal(body)
	if err != nil {
		return result
	}
	return string(out)
}
