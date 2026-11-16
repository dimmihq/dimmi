/*
|--------------------------------------------------------------------------
| LLM Request Types
|--------------------------------------------------------------------------
|
| The outbound side of the contract: a request is one system block
| plus a transcript, optionally with tools, a response schema and
| per-call timeouts.
|
*/

package contracts

import "time"

/*
ChatRequest is one completion call. Static system blocks come first so
providers that cache prompts can reuse them; dynamic context comes
last. Effort asks a reasoning model to think less or more; empty
leaves the provider's default, and models that do not reason ignore it.
Schema, when set, makes the answer one JSON object the provider
enforces; a driver that cannot enforce it refuses the call with
ErrSchemaUnsupported and never falls back to asking in the prompt.
*/
type ChatRequest struct {
	Model           string
	System          []SystemBlock
	Messages        []ChatMessage
	Tools           []ToolSpec
	ToolChoice      ToolChoice
	MaxOutputTokens int
	Temperature     *float64
	StopSequences   []string
	Schema          *ResponseSchema
	CacheHint       bool
	Metadata        map[string]string
	Timeout         StepTimeouts
	Effort          Effort
}

/*
Effort is how much a reasoning model thinks before it answers. Lower
effort answers sooner and spends fewer tokens.
*/
type Effort string

/*
The effort levels a request can ask for.
*/
const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
)

/*
SystemBlock is one system-prompt segment. Cacheable blocks hold only
typed, Dimmi-controlled text so they can be marked for prompt caching.
*/
type SystemBlock struct {
	Text      string
	Cacheable bool
}

/*
ChatRole names who produced a message part.
*/
type ChatRole string

/*
The roles a message can carry.
*/
const (
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
	ChatRoleTool      ChatRole = "tool"
)

/*
ChatMessage is one turn of a conversation. Customer messages carry the
user role; teammate and agent messages carry the assistant role with a
name prefix; tool results carry the tool role. Cacheable marks the end
of history that stays the same from call to call, which a provider with
explicit caching may cache up to.
*/
type ChatMessage struct {
	Role       ChatRole
	Parts      []ContentPart
	ToolCalls  []ToolCall
	ToolCallID string
	ToolError  bool
	Cacheable  bool
	// Sources are knowledge passages the message carries for a model
	// that cites them itself (LLMCapabilities.Citations): a search's
	// results on a tool message, carried knowledge on a user message.
	// A driver sends them as its own citable blocks beside the parts.
	Sources []ChatSource
}

/*
ChatSource is one knowledge passage a model can cite. ID is the number
the pipeline cites it by; a driver that cites natively writes "[ID]"
after each stretch of the reply the passage supports, so the reply
reads the same as one whose model wrote the markers itself, with the
difference that the provider, not the model, decides which passage a
marker names.
*/
type ChatSource struct {
	ID    string
	Title string
	URL   string
	Text  string
}

/*
ContentPart is one piece of a message's body. Text is the only kind at
this stage.
*/
type ContentPart struct {
	Text string
}

/*
StepTimeouts bound one provider call: time to the first token, idle
time between stream events, and the whole call.
*/
type StepTimeouts struct {
	FirstToken time.Duration
	Idle       time.Duration
	Total      time.Duration
}
