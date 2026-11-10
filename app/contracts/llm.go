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
		Capabilities answers what a model can do from the driver's
		static table plus connection overrides, never from the
		network.
	*/
	Capabilities(model string) LLMCapabilities
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
	/*
		Next blocks for the next event and returns io.EOF after the
		final done event.
	*/
	Next(ctx context.Context) (StreamEvent, error)

	/*
		Close releases the connection early; it is safe to call more
		than once.
	*/
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
LLMCapabilities says what a model supports, from the driver's static
table plus connection overrides. JSONSchema means the provider enforces
a request's ResponseSchema, so the answer is typed.
*/
type LLMCapabilities struct {
	Tools           bool
	Streaming       bool
	SystemPrompt    bool
	JSONSchema      bool
	PromptCache     bool
	ContextTokens   int
	MaxOutputTokens int
}
