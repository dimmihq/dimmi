/*
|--------------------------------------------------------------------------
| Fake LLM
|--------------------------------------------------------------------------
|
| Tests script a whole conversation with a model without spending a
| token: canned answers, streams, tool calls, errors and delays, and
| afterwards they read back exactly what the pipeline sent.
|
*/

package fake

import (
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Step is one scripted response the fake serves in order.
*/
type Step struct {
	Text      string
	Deltas    []string
	ToolCalls []contracts.ToolCall
	Usage     contracts.LLMUsage
	Err       error
	Delay     time.Duration
}

/*
LLM is a scripted contracts.LLM. Steps are served in order; when they
run out the last one repeats, and with no steps at all the fake
answers a fixed empty text.
*/
type LLM struct {
	mu       sync.Mutex
	provider contracts.LLMProvider
	steps    []Step
	sent     []contracts.ChatRequest
	models   []contracts.LLMModel
	caps     contracts.LLMCapabilities
	listErr  error
}

/*
New returns a fake for the given provider name.
*/
func New(provider contracts.LLMProvider) *LLM {
	return &LLM{provider: provider, caps: contracts.LLMCapabilities{
		Tools: true, Streaming: true, SystemPrompt: true, JSONMode: true, PromptCache: true,
		ContextTokens: 200_000, MaxOutputTokens: 8_192,
	}}
}

/*
Script sets the steps served in order.
*/
func (l *LLM) Script(steps ...Step) *LLM {
	l.steps = steps
	return l
}

/*
Models sets what ListModels answers.
*/
func (l *LLM) Models(models ...contracts.LLMModel) *LLM {
	l.models = models
	return l
}

/*
CapabilitiesOf sets the static capability answer.
*/
func (l *LLM) CapabilitiesOf(caps contracts.LLMCapabilities) *LLM {
	l.caps = caps
	return l
}

/*
Sent returns every request the fake received.
*/
func (l *LLM) Sent() []contracts.ChatRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]contracts.ChatRequest, len(l.sent))
	copy(out, l.sent)
	return out
}

/*
AssertSent reports whether at least one received request satisfies
the predicate.
*/
func (l *LLM) AssertSent(fn func(contracts.ChatRequest) bool) bool {
	return slices.ContainsFunc(l.Sent(), fn)
}

/*
Provider names the scripted provider.
*/
func (l *LLM) Provider() contracts.LLMProvider { return l.provider }

/*
Chat serves the next step as a finished response.
*/
func (l *LLM) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	step := l.next(req)

	if step.Delay > 0 {
		if err := sleep(ctx, step.Delay); err != nil {
			return nil, &contracts.LLMError{Kind: contracts.LLMErrorCanceled, Message: err.Error()}
		}
	}

	if step.Err != nil {
		return nil, step.Err
	}

	usage := step.Usage
	usage.Reported = true

	return &contracts.ChatResponse{
		Text:       step.Text,
		ToolCalls:  step.ToolCalls,
		StopReason: contracts.StopReasonEnd,
		Usage:      usage,
		Model:      req.Model,
	}, nil
}

/*
Stream serves the next step as a stream of deltas.
*/
func (l *LLM) Stream(_ context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	step := l.next(req)

	if step.Err != nil {
		return nil, step.Err
	}

	events := []contracts.StreamEvent{}
	deltas := step.Deltas

	if len(deltas) == 0 && step.Text != "" {
		deltas = []string{step.Text}
	}

	for _, d := range deltas {
		events = append(events, contracts.StreamEvent{Kind: contracts.StreamEventTextDelta, Text: d})
	}

	for i := range step.ToolCalls {
		call := step.ToolCalls[i]
		events = append(events, contracts.StreamEvent{Kind: contracts.StreamEventToolCallDone, ToolCall: &call})
	}

	usage := step.Usage
	usage.Reported = true
	events = append(events,
		contracts.StreamEvent{Kind: contracts.StreamEventUsage, Usage: &usage},
		contracts.StreamEvent{Kind: contracts.StreamEventDone, StopReason: contracts.StopReasonEnd},
	)

	return &stream{events: events}, nil
}

/*
ListModels serves the scripted list or the supported error.
*/
func (l *LLM) ListModels(_ context.Context) ([]contracts.LLMModel, error) {
	if l.listErr != nil {
		return nil, l.listErr
	}
	return l.models, nil
}

/*
Capabilities serves the scripted capabilities.
*/
func (l *LLM) Capabilities(_ string) contracts.LLMCapabilities { return l.caps }

/*
next records the request and pops the next scripted step.
*/
func (l *LLM) next(req contracts.ChatRequest) Step {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sent = append(l.sent, req)

	if len(l.steps) == 0 {
		return Step{Text: ""}
	}

	step := l.steps[0]
	if len(l.steps) > 1 {
		l.steps = l.steps[1:]
	}
	return step
}

/*
sleep waits or dies with the context.
*/
func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-clock.System().After(d):
		return nil
	}
}

/*
stream replays scripted events.
*/
type stream struct {
	events []contracts.StreamEvent
	pos    int
}

/*
Next replays the next event, ending with io.EOF.
*/
func (s *stream) Next(_ context.Context) (contracts.StreamEvent, error) {
	if s.pos >= len(s.events) {
		return contracts.StreamEvent{}, io.EOF
	}
	ev := s.events[s.pos]
	s.pos++
	return ev, nil
}

/*
Close ends the replay.
*/
func (s *stream) Close() error { return nil }

/*
ErrUnscripted marks a call the script did not answer.
*/
var ErrUnscripted = errors.New("llm fake: no scripted step")
