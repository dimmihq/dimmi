/*
|--------------------------------------------------------------------------
| Search Before The First Call
|--------------------------------------------------------------------------
|
| Most replies search the knowledge once with the customer's own words,
| which cost a whole model call just to decide to search. With the
| agent's up-front search on, the turn searches with the customer's
| message before the first model call, through the same tool, audience
| filters and limits, and hands the results to the model as if it had
| searched itself; it keeps the search tool for a refined second search.
| A message that is only a greeting or thanks is not searched.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
prefetchQueryChars caps the query taken from the customer's message.
*/
const prefetchQueryChars = 300

/*
prefetchNote follows the up-front search's results: they were found with
the customer's own words, which may name a neighboring feature, so the
model checks they answer the exact question before relying on them.
*/
const prefetchNote = "\n\n(Found with the customer's own words. Check these answer exactly what they asked, " +
	"for the product area they mean; if not, search again with the product's own terms before answering.)"

/*
prefetchCallID names the up-front search's call in the turn.
*/
const prefetchCallID = "prefetch_search"

/*
prefetch searches with the customer's message before the first model
call and adds the search and its results to the turn.
*/
func (r *Runner) prefetch(ctx context.Context, st *run) {
	if !st.env.Agent.PrefetchSearch || st.loop == nil || st.actions.followUp != nil {
		return
	}
	query := prefetchQuery(r.redactFor(st.env, st.env.pendingText()))
	if query == "" {
		return
	}
	args, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return
	}
	call := contracts.ToolCall{ID: prefetchCallID, Name: toolSearchKnowledge, Arguments: args}
	result, failed := r.callTool(ctx, st, call)
	if failed {
		return
	}
	st.prefetched = true
	sources := sourcesOf(st, call, false)
	if sources == nil {
		st.loop.messages = append(st.loop.messages,
			contracts.ChatMessage{Role: contracts.ChatRoleAssistant, ToolCalls: []contracts.ToolCall{call}},
			contracts.ChatMessage{Role: contracts.ChatRoleTool, ToolCallID: call.ID, Parts: textParts(result + prefetchNote)},
		)
		return
	}
	// Cited passages fill the tool result on their own, so the note
	// follows it as text in the same turn.
	st.loop.messages = append(st.loop.messages,
		contracts.ChatMessage{Role: contracts.ChatRoleAssistant, ToolCalls: []contracts.ToolCall{call}},
		contracts.ChatMessage{
			Role: contracts.ChatRoleTool, ToolCallID: call.ID, Parts: textParts(result), Sources: sources,
		},
		contracts.ChatMessage{Role: contracts.ChatRoleUser, Parts: textParts(strings.TrimSpace(prefetchNote))},
	)
}

/*
prefetchQuery is the query taken from the customer's message, empty
when the message is only a greeting or thanks.
*/
func prefetchQuery(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || smallTalk(text) {
		return ""
	}
	if runes := []rune(text); len(runes) > prefetchQueryChars {
		text = string(runes[:prefetchQueryChars])
	}
	return text
}

/*
smallTalk reports whether a short message only greets, thanks or
agrees.
*/
func smallTalk(text string) bool {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\''
	})
	if len(words) == 0 || len(words) > 6 {
		return len(words) == 0
	}
	for _, w := range words {
		if !smallTalkWords()[w] {
			return false
		}
	}
	return true
}

/*
smallTalkWords are the words a greeting, thanks or agreement is made
of.
*/
func smallTalkWords() map[string]bool {
	out := map[string]bool{}
	for w := range strings.FieldsSeq("hi hello hey hiya morning good afternoon evening thanks thank you thx ty " +
		"cheers ok okay k great perfect awesome cool got it that's that is all bye goodbye much so very " +
		"appreciate appreciated yes yep no nope sure fine the there team again lovely brilliant") {
		out[w] = true
	}
	return out
}

/*
compactSearch writes search results as short blocks: the number, the
title and link on one line, the text below. A passage this run already
showed the model is pointed to instead of sent again.
*/
func compactSearch(results []searchResult, shown map[int]bool) string {
	if len(results) == 0 {
		return "no results"
	}
	var b strings.Builder
	for _, res := range results {
		if shown[res.ID] {
			b.WriteString("[" + strconv.Itoa(res.ID) + "] " + res.Title + " (shown above)\n\n")
			continue
		}
		shown[res.ID] = true
		b.WriteString("[" + strconv.Itoa(res.ID) + "] " + res.Title)
		if res.URL != "" {
			b.WriteString(" " + res.URL)
		}
		b.WriteString("\n" + strings.TrimSpace(res.Text) + "\n\n")
	}
	return strings.TrimSpace(b.String())
}
