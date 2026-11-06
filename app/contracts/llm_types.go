/*
|--------------------------------------------------------------------------
| LLM Request Types
|--------------------------------------------------------------------------
|
| The outbound side of the contract: a request is one system block
| plus a transcript, optionally with tools, a response format and
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
	ResponseFormat  ResponseFormat
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
name prefix; tool results carry the tool role.
*/
type ChatMessage struct {
	Role       ChatRole
	Parts      []ContentPart
	ToolCalls  []ToolCall
	ToolCallID string
	ToolError  bool
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
