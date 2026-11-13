/*
|--------------------------------------------------------------------------
| Token Ledger
|--------------------------------------------------------------------------
|
| What a model call's input is made of, in the order the provider reads
| it: the fixed prefix (tool definitions and system prompt), the
| conversation's history, the newest message, tool definitions loaded
| during the turn, knowledge search results and other tool results.
| Providers report only totals, and a cache read always covers the start
| of the input, so a call's reported fresh, cached and written tokens
| are split across these parts in order: the cached tokens go to the
| first parts, the written ones next, the fresh ones last, each part's
| size taken from its share of the input's characters.
|
*/

package ledger

import (
	"encoding/json"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
The parts of a call's input.
*/
const (
	Fixed   = "fixed"
	History = "history"
	Newest  = "newest"
	Loaded  = "loaded_tools"
	Search  = "search"
	Results = "tool_results"
	Calls   = "tool_calls"
)

/*
Tool names the ledger tells apart.
*/
const (
	loadTool   = "load_tools"
	searchTool = "search_knowledge"
)

/*
Segment is one run of the input that belongs to one part, in order.
*/
type Segment struct {
	Part  string `json:"p"`
	Chars int    `json:"c"`
}

/*
Of splits a request's input into segments in the order the provider
reads it, given the index of the message that started the turn: the
messages before it are history, the ones after it the turn's own tool
calls and results.
*/
func Of(req contracts.ChatRequest, newest int) []Segment {
	var out []Segment
	add := func(part string, chars int) {
		if chars <= 0 {
			return
		}
		if n := len(out); n > 0 && out[n-1].Part == part {
			out[n-1].Chars += chars
			return
		}
		out = append(out, Segment{Part: part, Chars: chars})
	}
	for _, t := range req.Tools {
		add(Fixed, len(t.Name)+len(t.Description)+len(t.Schema))
	}
	for _, s := range req.System {
		add(Fixed, len(s.Text))
	}
	names := map[string]string{}
	for i, m := range req.Messages {
		for _, c := range m.ToolCalls {
			names[c.ID] = c.Name
		}
		add(partOf(m, i, newest, names), size(m))
	}
	return out
}

/*
partOf names the part a message belongs to.
*/
func partOf(m contracts.ChatMessage, i, newest int, names map[string]string) string {
	switch {
	case i < newest:
		return History
	case i == newest:
		return Newest
	case m.Role != contracts.ChatRoleTool:
		return Calls
	}
	switch names[m.ToolCallID] {
	case loadTool:
		return Loaded
	case searchTool:
		return Search
	default:
		return Results
	}
}

/*
size is a message's characters: its text and its tool calls.
*/
func size(m contracts.ChatMessage) int {
	n := 0
	for _, p := range m.Parts {
		n += len(p.Text)
	}
	for _, c := range m.ToolCalls {
		n += len(c.Name) + len(c.Arguments)
	}
	return n
}

/*
Encode is the segments as stored on a step.
*/
func Encode(segments []Segment) json.RawMessage {
	if len(segments) == 0 {
		return nil
	}
	out, err := json.Marshal(segments)
	if err != nil {
		return nil
	}
	return out
}

/*
Tokens are one part's input tokens by how the provider billed them.
*/
type Tokens struct {
	Cached  int64 `json:"cached"`
	Written int64 `json:"written"`
	Fresh   int64 `json:"fresh"`
}

/*
Total is all of a part's input tokens.
*/
func (t Tokens) Total() int64 {
	return t.Cached + t.Written + t.Fresh
}

/*
Split divides a call's reported input across its segments: each part
gets tokens in proportion to its characters, and the cached tokens fill
the parts from the start, then the written ones, then the fresh ones.
*/
func Split(segments []Segment, u contracts.LLMUsage) map[string]Tokens {
	out := map[string]Tokens{}
	total := u.InputTokens + u.CachedInputTokens + u.CacheWriteTokens
	chars := 0
	for _, s := range segments {
		chars += s.Chars
	}
	if total <= 0 || chars == 0 {
		return out
	}
	cached, written := u.CachedInputTokens, u.CacheWriteTokens
	given := int64(0)
	for i, s := range segments {
		share := total * int64(s.Chars) / int64(chars)
		if i == len(segments)-1 {
			share = total - given
		}
		given += share
		t := out[s.Part]
		take := min(share, cached)
		t.Cached, cached, share = t.Cached+take, cached-take, share-take
		take = min(share, written)
		t.Written, written, share = t.Written+take, written-take, share-take
		t.Fresh += share
		out[s.Part] = t
	}
	return out
}
