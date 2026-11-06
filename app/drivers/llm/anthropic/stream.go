/*
|--------------------------------------------------------------------------
| Anthropic Stream
|--------------------------------------------------------------------------
|
| The SDK accumulates the wire events into a message; this emits our
| deltas as they arrive and a tool call only when its input is whole.
|
*/

package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	anth "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
chatStream replays Anthropic events as contract events.
*/
type chatStream struct {
	mu     sync.Mutex
	stream *ssestream.Stream[anth.MessageStreamEventUnion]
	msg    anth.Message
	pend   []contracts.StreamEvent
	done   bool
	closed bool
}

/*
newChatStream wraps the SDK stream.
*/
func newChatStream(stream *ssestream.Stream[anth.MessageStreamEventUnion]) *chatStream {
	return &chatStream{stream: stream}
}

/*
Next blocks for the next event, ending with io.EOF.
*/
func (s *chatStream) Next(_ context.Context) (contracts.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		if len(s.pend) > 0 {
			return s.pop(), nil
		}
		if s.done {
			return contracts.StreamEvent{}, io.EOF
		}

		for s.stream.Next() {
			s.handleEvent(s.stream.Current())
		}

		if err := s.stream.Err(); err != nil {
			return contracts.StreamEvent{}, MapError(err)
		}

		if len(s.pend) == 0 {
			s.done = true
		}
	}
}

/*
handleEvent accumulates one SDK event and queues ours.
*/
func (s *chatStream) handleEvent(event anth.MessageStreamEventUnion) {
	if err := s.msg.Accumulate(event); err != nil {
		return
	}

	switch ev := event.AsAny().(type) {
	case anth.ContentBlockDeltaEvent:
		if delta, ok := ev.Delta.AsAny().(anth.TextDelta); ok && delta.Text != "" {
			s.pend = append(s.pend, contracts.StreamEvent{
				Kind: contracts.StreamEventTextDelta, Text: delta.Text,
			})
		}
	case anth.MessageDeltaEvent:
		if ev.Usage.OutputTokens > 0 {
			usage := s.usage()
			s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventUsage, Usage: usage})
		}
	case anth.MessageStopEvent:
		s.finishPending()
		s.pend = append(s.pend, contracts.StreamEvent{
			Kind: contracts.StreamEventDone, StopReason: mapStop(s.msg.StopReason),
		})
	}
}

/*
finishPending emits assembled tool calls at stream end.
*/
func (s *chatStream) finishPending() {
	for _, block := range s.msg.Content {
		if tool, ok := block.AsAny().(anth.ToolUseBlock); ok {
			args, _ := json.Marshal(tool.Input)
			call := contracts.ToolCall{ID: tool.ID, Name: tool.Name, Arguments: args}
			s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventToolCallDone, ToolCall: &call})
		}
	}
}

/*
usage maps the accumulated message usage.
*/
func (s *chatStream) usage() *contracts.LLMUsage {
	return &contracts.LLMUsage{
		InputTokens:       s.msg.Usage.InputTokens,
		CachedInputTokens: s.msg.Usage.CacheReadInputTokens,
		CacheWriteTokens:  s.msg.Usage.CacheCreationInputTokens,
		OutputTokens:      s.msg.Usage.OutputTokens,
		Reported:          true,
	}
}

/*
pop returns the first queued event.
*/
func (s *chatStream) pop() contracts.StreamEvent {
	ev := s.pend[0]
	s.pend = s.pend[1:]
	return ev
}

/*
Close ends the stream.
*/
func (s *chatStream) Close() error {
	s.closed = true
	return s.stream.Close()
}
