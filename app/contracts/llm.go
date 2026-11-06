/*
|--------------------------------------------------------------------------
| LLM Contract
|--------------------------------------------------------------------------
|
| Every language model the agent talks to, whatever company serves it,
| is one configured connection behind this interface: the built-in
| provider, a subscription sign-in, or a key the workspace pasted. The
| pipeline never sees a provider SDK; a driver answers for one
| connection and is safe for concurrent use.
|
*/

package contracts

import (
	"context"
	"encoding/json"
	"time"
)

/*
LLMProvider names the driver behind a connection.
*/
type LLMProvider string

/*
The providers a connection can point at today. New drivers register
their own value; the database only checks a value's shape.
*/
const (
	LLMProviderDimmi            LLMProvider = "dimmi"
	LLMProviderAnthropic        LLMProvider = "anthropic"
	LLMProviderOpenAI           LLMProvider = "openai"
	LLMProviderChatGPT          LLMProvider = "chatgpt"
	LLMProviderEngy             LLMProvider = "engy"
	LLMProviderOpenAICompatible LLMProvider = "openai_compatible"
)

/*
LLM is one configured provider connection bound to one base URL. It is
built per use by the manager from a provider key row and stays safe for
concurrent use.
*/
type LLM interface {
	/*
		Provider names the driver behind the connection.
	*/
	Provider() LLMProvider

	/*
		Chat runs one non-streaming completion.
	*/
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	/*
		Stream runs one streaming completion. The caller closes the
		stream; errors before the first event come back from Stream,
		mid-stream errors from Next.
	*/
	Stream(ctx context.Context, req ChatRequest) (ChatStream, error)

	/*
		ListModels returns the models the connection can call, or
		ErrLLMNotSupported when the provider has no list endpoint and
		the caller falls back to Probe.
	*/
	ListModels(ctx context.Context) ([]LLMModel, error)

	/*
		Probe sends the smallest possible request to a model. It costs
		a few real tokens; it powers the test button and the
		validation fallback.
	*/
	Probe(ctx context.Context, model string) (*ProbeResult, error)

	/*
		Capabilities answers what a model can do from the driver's
		static table plus connection overrides, never from the
		network.
	*/
	Capabilities(model string) LLMCapabilities
}

/*
ChatRequest is one completion call. Static system blocks come first so
providers that cache prompts can reuse them; dynamic context comes
last.
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
}

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
ToolSpec describes one tool the model may call. Names match the agent
tool rules so they stay valid for every provider.
*/
type ToolSpec struct {
	Name        string
	Description string
	Schema      json.RawMessage
}

/*
ToolCall is one invocation the model asked for.
*/
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

/*
ToolChoice tells the model how it may use tools.
*/
type ToolChoice string

/*
The tool choices. Forced tool use is never sent: some models reject it.
*/
const (
	ToolChoiceAuto ToolChoice = "auto"
	ToolChoiceNone ToolChoice = "none"
)

/*
ResponseFormat names the answer shape the caller needs.
*/
type ResponseFormat string

/*
The answer shapes. JSON is used by the classifier; drivers map it to
the native mode or a prompt suffix where none exists.
*/
const (
	ResponseFormatText ResponseFormat = "text"
	ResponseFormatJSON ResponseFormat = "json"
)

/*
StepTimeouts bound one provider call: time to the first token, idle
time between stream events, and the whole call.
*/
type StepTimeouts struct {
	FirstToken time.Duration
	Idle       time.Duration
	Total      time.Duration
}

/*
ChatResponse is one finished completion.
*/
type ChatResponse struct {
	Text       string
	ToolCalls  []ToolCall
	StopReason StopReason
	Usage      LLMUsage
	Model      string
	RequestID  string
	Latency    time.Duration
}

/*
StopReason names why a completion ended.
*/
type StopReason string

/*
The stop reasons, mapped from each provider's own vocabulary.
*/
const (
	StopReasonEnd           StopReason = "end"
	StopReasonMaxTokens     StopReason = "max_tokens"
	StopReasonToolUse       StopReason = "tool_use"
	StopReasonStopSequence  StopReason = "stop_sequence"
	StopReasonContentFilter StopReason = "content_filter"
	StopReasonOther         StopReason = "other"
)

/*
ChatStream is a live completion. Next blocks for the next event and
returns io.EOF after the final done event.
*/
type ChatStream interface {
	Next(ctx context.Context) (StreamEvent, error)
	Close() error
}

/*
StreamEventKind names the kind of one stream event.
*/
type StreamEventKind string

/*
The stream event kinds.
*/
const (
	StreamEventTextDelta    StreamEventKind = "text_delta"
	StreamEventToolCallDone StreamEventKind = "tool_call_done"
	StreamEventUsage        StreamEventKind = "usage"
	StreamEventDone         StreamEventKind = "done"
)

/*
StreamEvent is one streamed piece of a completion.
*/
type StreamEvent struct {
	Kind       StreamEventKind
	Text       string
	ToolCall   *ToolCall
	Usage      *LLMUsage
	StopReason StopReason
}

/*
LLMUsage is the token accounting of one call, including the cache
tokens some providers report. Reported is false when the provider sent
no usage; the pipeline then estimates and flags the step.
*/
type LLMUsage struct {
	InputTokens       int64
	CachedInputTokens int64
	CacheWriteTokens  int64
	OutputTokens      int64
	ReasoningTokens   int64
	Reported          bool
}

/*
LLMModel describes one model a connection can call.
*/
type LLMModel struct {
	ID            string
	DisplayName   string
	ContextTokens int
	Deprecated    bool
}

/*
ProbeResult is what a tiny request answered: the model reached and its
cost in latency and tokens.
*/
type ProbeResult struct {
	Model   string
	Latency time.Duration
	Usage   LLMUsage
}

/*
LLMCapabilities says what a model supports, from the driver's static
table plus connection overrides.
*/
type LLMCapabilities struct {
	Tools           bool
	Streaming       bool
	SystemPrompt    bool
	JSONMode        bool
	PromptCache     bool
	ContextTokens   int
	MaxOutputTokens int
}
