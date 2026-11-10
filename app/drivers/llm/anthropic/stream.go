/*
|--------------------------------------------------------------------------
| Anthropic Stream
|--------------------------------------------------------------------------
|
| Each wire event becomes contract events the moment it lands: text
| deltas pass straight through, a tool call is emitted when its block
| closes and its input is whole, usage and the stop reason close the
| stream. A stream the provider cuts before its stop event is an
| error, never a silent end. When the request asked for a typed
| answer, the forced tool's input arrives as text, never as a call.
|
*/

package anthropic

import (
	"context"
	"io"
	"sync"

	anth "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
chatStream replays Anthropic events as contract events. Its answer
field names the forced tool of a typed answer, empty when there is
none.
*/
type chatStream struct {
	mu     sync.Mutex
	stream *ssestream.Stream[anth.MessageStreamEventUnion]
	clock  clock.Clock
	answer string
	msg    anth.Message
	pend   []contracts.StreamEvent
	done   bool
}

/*
newChatStream wraps the SDK stream.
*/
func newChatStream(
	stream *ssestream.Stream[anth.MessageStreamEventUnion], clk clock.Clock, answer string,
) *chatStream {
	return &chatStream{stream: stream, clock: clk, answer: answer}
}

/*
prime reads the first event, surfacing a refused request or an error
the provider sent before any content.
*/
func (s *chatStream) prime() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.advance()
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
			return MapError(err, s.clock.Now())
		}
		return &contracts.LLMError{Kind: contracts.LLMErrorNetwork, Message: "stream ended before the reply finished"}
	}

	event := s.stream.Current()
	if err := s.msg.Accumulate(event); err != nil {
		return &contracts.LLMError{Kind: contracts.LLMErrorServer, Message: "malformed stream event"}
	}
	s.handle(event)
	return nil
}

/*
handle queues the contract events one SDK event carries.
*/
func (s *chatStream) handle(event anth.MessageStreamEventUnion) {
	switch event.Type {
	case "content_block_delta":
		if event.Delta.Type == "text_delta" && event.Delta.Text != "" && s.answer == "" {
			s.pend = append(s.pend, contracts.StreamEvent{
				Kind: contracts.StreamEventTextDelta, Text: event.Delta.Text,
			})
		}
	case "content_block_stop":
		s.blockDone(s.msg.Content[event.Index])
	case "message_stop":
		usage := usageOf(s.msg.Usage)
		stop := mapStop(s.msg.StopReason)
		if s.answer != "" && stop == contracts.StopReasonToolUse {
			stop = contracts.StopReasonEnd
		}
		s.pend = append(s.pend,
			contracts.StreamEvent{Kind: contracts.StreamEventUsage, Usage: &usage},
			contracts.StreamEvent{Kind: contracts.StreamEventDone, StopReason: stop},
		)
		s.done = true
	}
}

/*
blockDone queues a finished tool call, or for a typed answer the forced
tool's input as text.
*/
func (s *chatStream) blockDone(block anth.ContentBlockUnion) {
	if block.Type != "tool_use" {
		return
	}
	call := toolCall(block)
	if s.answer != "" {
		text := string(call.Arguments)
		s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventTextDelta, Text: text})
		return
	}
	s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventToolCallDone, ToolCall: &call})
}

/*
Close releases the connection; it never waits for a blocked Next,
which then fails on the closed body.
*/
func (s *chatStream) Close() error {
	return s.stream.Close()
}
