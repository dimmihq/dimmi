/*
|--------------------------------------------------------------------------
| OpenAI Stream
|--------------------------------------------------------------------------
|
| Each wire chunk becomes events as it arrives: text at once, tool
| calls whole once the model finishes them, usage when the provider
| reports it, and a final done carrying why the model stopped. Nothing
| is buffered beyond the chunk being read, so the first token reaches
| the customer as soon as the provider sends it.
|
*/

package openai

import (
	"context"
	"io"
	"slices"
	"strings"
	"sync"

	oai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
chatStream replays SDK chunks as contract events.
*/
type chatStream struct {
	mu     sync.Mutex
	stream *ssestream.Stream[oai.ChatCompletionChunk]
	clock  clock.Clock
	calls  map[int64]*pendingCall
	pend   []contracts.StreamEvent
	stop   contracts.StopReason
	done   bool
}

/*
pendingCall is a tool call whose arguments are still arriving.
*/
type pendingCall struct {
	id   string
	name string
	args strings.Builder
}

/*
newChatStream wraps the SDK stream.
*/
func newChatStream(stream *ssestream.Stream[oai.ChatCompletionChunk], clk clock.Clock) *chatStream {
	return &chatStream{stream: stream, clock: clk, calls: map[int64]*pendingCall{}}
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
advance reads one chunk, or finishes the stream when the provider
ended it.
*/
func (s *chatStream) advance() error {
	if s.stream.Next() {
		s.handle(s.stream.Current())
		return nil
	}
	if err := s.stream.Err(); err != nil {
		return MapError(err, s.clock.Now())
	}
	s.finish()
	return nil
}

/*
handle queues the events one chunk carries.
*/
func (s *chatStream) handle(chunk oai.ChatCompletionChunk) {
	for _, choice := range chunk.Choices {
		if choice.Index != 0 {
			continue
		}
		if choice.Delta.Content != "" {
			s.pend = append(s.pend, contracts.StreamEvent{
				Kind: contracts.StreamEventTextDelta, Text: choice.Delta.Content,
			})
		}
		for _, delta := range choice.Delta.ToolCalls {
			s.collect(delta)
		}
		if choice.FinishReason != "" {
			s.stop = MapFinish(choice.FinishReason)
			s.flushCalls()
		}
	}

	if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
		usage := usageOf(chunk.Usage)
		s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventUsage, Usage: &usage})
	}
}

/*
collect appends one tool-call fragment to its call.
*/
func (s *chatStream) collect(delta oai.ChatCompletionChunkChoiceDeltaToolCall) {
	call, ok := s.calls[delta.Index]
	if !ok {
		call = &pendingCall{}
		s.calls[delta.Index] = call
	}
	if delta.ID != "" {
		call.id = delta.ID
	}
	if delta.Function.Name != "" {
		call.name = delta.Function.Name
	}
	call.args.WriteString(delta.Function.Arguments)
}

/*
flushCalls emits every collected tool call in index order.
*/
func (s *chatStream) flushCalls() {
	indexes := make([]int64, 0, len(s.calls))
	for i := range s.calls {
		indexes = append(indexes, i)
	}
	slices.Sort(indexes)

	for _, i := range indexes {
		call := s.calls[i]
		done := contracts.ToolCall{ID: call.id, Name: call.name, Arguments: rawArgs(call.args.String())}
		s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventToolCallDone, ToolCall: &done})
	}
	if len(indexes) > 0 && s.stop == contracts.StopReasonEnd {
		s.stop = contracts.StopReasonToolUse
	}
	clear(s.calls)
}

/*
finish emits calls the provider never closed and the final done.
*/
func (s *chatStream) finish() {
	if len(s.calls) > 0 {
		s.flushCalls()
	}
	if s.stop == "" {
		s.stop = contracts.StopReasonOther
	}
	s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventDone, StopReason: s.stop})
	s.done = true
}

/*
Close releases the connection; it never waits for a blocked Next,
which then fails on the closed body.
*/
func (s *chatStream) Close() error {
	return s.stream.Close()
}
