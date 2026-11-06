/*
|--------------------------------------------------------------------------
| ChatGPT Stream
|--------------------------------------------------------------------------
|
| Responses events become contract events as they land: text deltas
| pass through, a function call is emitted whole when its output item
| is done, carrying the call id and name the item announced. The
| terminal event decides the ending: completed ends normally or on a
| tool call, incomplete ends on the reason the plan gave, failed and
| error become typed errors. A stream that stops without one of them
| is an error, never a silent end.
|
*/

package chatgpt

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/openai/openai-go/v3/packages/ssestream"
	"github.com/openai/openai-go/v3/responses"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/llm/openai"
)

/*
chatStream replays response events as contract events.
*/
type chatStream struct {
	mu     sync.Mutex
	stream *ssestream.Stream[responses.ResponseStreamEventUnion]
	clock  clock.Clock
	pend   []contracts.StreamEvent
	calls  int
	done   bool
}

/*
newChatStream wraps the SDK stream.
*/
func newChatStream(stream *ssestream.Stream[responses.ResponseStreamEventUnion], clk clock.Clock) *chatStream {
	return &chatStream{stream: stream, clock: clk}
}

/*
Next blocks for the next event, ending with io.EOF after done.
*/
func (s *chatStream) Next(_ context.Context) (contracts.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for len(s.pend) == 0 {
		if s.done {
			return contracts.StreamEvent{}, io.EOF
		}
		if err := s.advance(); err != nil {
			s.done = true
			return contracts.StreamEvent{}, err
		}
	}

	ev := s.pend[0]
	s.pend = s.pend[1:]
	return ev, nil
}

/*
advance reads one SDK event and queues what it carries.
*/
func (s *chatStream) advance() error {
	if !s.stream.Next() {
		if err := s.stream.Err(); err != nil {
			return openai.MapError(err, s.clock.Now())
		}
		return &contracts.LLMError{Kind: contracts.LLMErrorNetwork, Message: "stream ended before the reply finished"}
	}
	return s.handle(s.stream.Current())
}

/*
handle queues the contract events one response event carries.
*/
func (s *chatStream) handle(event responses.ResponseStreamEventUnion) error {
	switch event.Type {
	case "response.output_text.delta":
		if event.Delta != "" {
			s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventTextDelta, Text: event.Delta})
		}
	case "response.output_item.done":
		s.itemDone(event.AsResponseOutputItemDone().Item)
	case "response.completed":
		stop := contracts.StopReasonEnd
		if s.calls > 0 {
			stop = contracts.StopReasonToolUse
		}
		s.finish(event.AsResponseCompleted().Response.Usage, stop)
	case "response.incomplete":
		res := event.AsResponseIncomplete().Response
		s.finish(res.Usage, incompleteReason(res.IncompleteDetails.Reason))
	case "response.failed":
		failure := event.AsResponseFailed().Response.Error
		return openai.CodeFailure(string(failure.Code), failure.Message)
	case "error":
		failure := event.AsError()
		return openai.CodeFailure(failure.Code, failure.Message)
	}
	return nil
}

/*
itemDone emits a finished function call with the id and name its
item carries.
*/
func (s *chatStream) itemDone(item responses.ResponseOutputItemUnion) {
	if item.Type != "function_call" {
		return
	}
	args := json.RawMessage(item.Arguments.OfString)
	if len(args) == 0 || !json.Valid(args) {
		args = json.RawMessage("{}")
	}
	call := contracts.ToolCall{ID: item.CallID, Name: item.Name, Arguments: args}
	s.calls++
	s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventToolCallDone, ToolCall: &call})
}

/*
finish queues usage and the final done.
*/
func (s *chatStream) finish(u responses.ResponseUsage, stop contracts.StopReason) {
	if u.InputTokens > 0 || u.OutputTokens > 0 {
		cached := u.InputTokensDetails.CachedTokens
		s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventUsage, Usage: &contracts.LLMUsage{
			InputTokens:       max(u.InputTokens-cached, 0),
			CachedInputTokens: cached,
			OutputTokens:      u.OutputTokens,
			ReasoningTokens:   u.OutputTokensDetails.ReasoningTokens,
			Reported:          true,
		}})
	}
	s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventDone, StopReason: stop})
	s.done = true
}

/*
incompleteReason maps why a response stopped short.
*/
func incompleteReason(reason string) contracts.StopReason {
	switch reason {
	case "max_output_tokens":
		return contracts.StopReasonMaxTokens
	case "content_filter":
		return contracts.StopReasonContentFilter
	}
	return contracts.StopReasonOther
}

/*
Close releases the connection; it never waits for a blocked Next,
which then fails on the closed body.
*/
func (s *chatStream) Close() error { return s.stream.Close() }
