/*
|--------------------------------------------------------------------------
| ChatGPT Stream
|--------------------------------------------------------------------------
|
| Response stream events become our events: text deltas pass through,
| finished function calls arrive whole, and usage lands with done.
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

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
chatStream replays response events as contract events.
*/
type chatStream struct {
	mu     sync.Mutex
	stream *ssestream.Stream[responses.ResponseStreamEventUnion]
	pend   []contracts.StreamEvent
	done   bool
}

/*
newChatStream wraps the SDK stream.
*/
func newChatStream(stream *ssestream.Stream[responses.ResponseStreamEventUnion], _ string) *chatStream {
	return &chatStream{stream: stream}
}

/*
Next blocks for the next event, ending with io.EOF.
*/
func (s *chatStream) Next(ctx context.Context) (contracts.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		if len(s.pend) > 0 {
			ev := s.pend[0]
			s.pend = s.pend[1:]
			return ev, nil
		}
		if s.done {
			return contracts.StreamEvent{}, io.EOF
		}

		for s.stream.Next() {
			event := s.stream.Current()

			switch ev := event.AsAny().(type) {
			case responses.ResponseTextDeltaEvent:
				if ev.Delta != "" {
					s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventTextDelta, Text: ev.Delta})
				}
			case responses.ResponseFunctionCallArgumentsDoneEvent:
				call := contracts.ToolCall{ID: ev.ItemID, Name: "", Arguments: json.RawMessage(ev.Arguments)}
				s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventToolCallDone, ToolCall: &call})
			case responses.ResponseCompletedEvent:
				if u := ev.Response.Usage; u.InputTokens > 0 || u.OutputTokens > 0 {
					usage := &contracts.LLMUsage{
						InputTokens:       u.InputTokens,
						CachedInputTokens: u.InputTokensDetails.CachedTokens,
						OutputTokens:      u.OutputTokens,
						ReasoningTokens:   u.OutputTokensDetails.ReasoningTokens,
						Reported:          true,
					}
					s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventUsage, Usage: usage})
				}
				s.pend = append(s.pend, contracts.StreamEvent{Kind: contracts.StreamEventDone, StopReason: contracts.StopReasonEnd})
			}
		}

		if err := s.stream.Err(); err != nil {
			return contracts.StreamEvent{}, mapError(err)
		}

		s.done = len(s.pend) == 0
	}
}

/*
Close ends the stream.
*/
func (s *chatStream) Close() error { return s.stream.Close() }
