/*
|--------------------------------------------------------------------------
| OpenAI Stream
|--------------------------------------------------------------------------
|
| The SDK accumulates the wire deltas; this turns each finished piece
| into one stream event: text as it arrives, a tool call only when its
| arguments are complete JSON, usage when the provider reports it.
|
*/

package openai

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	oai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
chatStream replays SDK deltas as contract events.
*/
type chatStream struct {
	mu     sync.Mutex
	stream *ssestream.Stream[oai.ChatCompletionChunk]
	acc    oai.ChatCompletionAccumulator
	model  string
	pend   []contracts.StreamEvent
	done   bool
}

/*
newChatStream wraps the SDK stream.
*/
func newChatStream(stream *ssestream.Stream[oai.ChatCompletionChunk], model string) *chatStream {
	return &chatStream{stream: stream, model: model}
}

/*
Next blocks for the next event, ending with io.EOF after done.
*/
func (s *chatStream) Next(ctx context.Context) (contracts.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pend) > 0 {
		return s.pop(), nil
	}

	for s.stream.Next() {
		chunk := s.stream.Current()
		s.acc.AddChunk(chunk)

		if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.Content) > 0 {
			s.pend = append(s.pend, contracts.StreamEvent{
				Kind: contracts.StreamEventTextDelta, Text: chunk.Choices[0].Delta.Content,
			})
		}

		if tool, ok := s.acc.JustFinishedToolCall(); ok {
			call := contracts.ToolCall{ID: tool.ID, Name: tool.Name, Arguments: json.RawMessage(tool.Arguments)}
			s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventToolCallDone, ToolCall: &call})
		}

		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			usage := &contracts.LLMUsage{
				InputTokens:       chunk.Usage.PromptTokens,
				CachedInputTokens: chunk.Usage.PromptTokensDetails.CachedTokens,
				OutputTokens:      chunk.Usage.CompletionTokens,
				ReasoningTokens:   chunk.Usage.CompletionTokensDetails.ReasoningTokens,
				Reported:          true,
			}
			s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventUsage, Usage: usage})
		}
	}

	if err := s.stream.Err(); err != nil {
		return contracts.StreamEvent{}, MapError(err)
	}

	if s.done {
		return contracts.StreamEvent{}, io.EOF
	}
	s.done = true

	if len(s.pend) > 0 {
		return s.pop(), nil
	}

	return contracts.StreamEvent{Kind: contracts.StreamEventDone, StopReason: contracts.StopReasonEnd}, nil
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
	s.done = true
	return s.stream.Close()
}
