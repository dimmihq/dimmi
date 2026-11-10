/*
|--------------------------------------------------------------------------
| Running Tool Calls
|--------------------------------------------------------------------------
|
| A model often asks for several lookups at once, such as reading five
| conversations. Reading tools touch nothing the others depend on, so a
| run of them goes out together and the teammate waits for the slowest
| instead of the sum. A writing tool runs alone, after every call before
| it has finished, since what it may do depends on what was read. The
| model reads the results, and the teammate sees the steps, in the order
| the calls were made.
|
*/

package operator

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
)

/*
pending is one tool call on its way: the call, the tool it names, when
it started, and what it returned.
*/
type pending struct {
	call    contracts.ToolCall
	tool    tool
	known   bool
	started time.Time
	out     result
	err     error
}

/*
callTools runs one model turn's tool calls and returns the messages
the model reads next, one per call in call order.
*/
func (e *Engine) callTools(ctx context.Context, s *session, calls []contracts.ToolCall) []contracts.ChatMessage {
	registry := e.registry(s)
	batch := make([]*pending, len(calls))
	for i, call := range calls {
		t, ok := registry[call.Name]
		batch[i] = &pending{call: call, tool: t, known: ok && e.offers(s, t)}
	}
	out := make([]contracts.ChatMessage, 0, len(calls))
	for start := 0; start < len(batch); {
		end := start + 1
		if !batch[start].writes() {
			for end < len(batch) && !batch[end].writes() {
				end++
			}
		}
		e.runGroup(ctx, s, batch[start:end])
		for _, p := range batch[start:end] {
			out = append(out, e.finishCall(ctx, s, p))
		}
		start = end
	}
	return out
}

/*
writes reports whether the call may change anything, so it runs alone.
*/
func (p *pending) writes() bool {
	return p.known && len(p.tool.writes) > 0
}

/*
runGroup runs a group of calls: a single call in place, several reading
calls together, each on its own copy of the session so they never
share state, and what they learned folded back once all are done.
*/
func (e *Engine) runGroup(ctx context.Context, s *session, group []*pending) {
	if len(group) == 1 {
		e.runCall(ctx, s, group[0])
		return
	}
	copies := make([]session, len(group))
	var wg sync.WaitGroup
	for i, p := range group {
		copies[i] = *s
		wg.Go(func() { e.runCall(ctx, &copies[i], p) })
	}
	wg.Wait()
	for i := range copies {
		s.tainted = s.tainted || copies[i].tainted
	}
}

/*
runCall runs one call's handler, or refuses a tool the run does not
offer.
*/
func (e *Engine) runCall(ctx context.Context, s *session, p *pending) {
	p.started = e.Clock.Now()
	p.err = errors.New("unknown tool " + p.call.Name)
	if p.known {
		p.out, p.err = p.tool.handle(ctx, s, p.call.Arguments)
	}
}

/*
finishCall counts and records a finished call, streams its step and
returns what the model reads, saying what went wrong when it failed.
*/
func (e *Engine) finishCall(ctx context.Context, s *session, p *pending) contracts.ChatMessage {
	s.toolCalls++
	out, err := p.out, p.err
	if out.step.Tool == "" {
		out.step = Step{Tool: p.call.Name, Summary: p.call.Name}
	}
	out.step.OK = err == nil
	if err != nil {
		out.text, out.step.Summary = refusal(err), out.step.Summary+": "+refusal(err)
	}
	s.steps = append(s.steps, out.step)
	e.publishStep(ctx, s, out.step)
	e.record(ctx, s, step{
		kind: enums.AIRunStepKindToolCall, name: p.call.Name, started: p.started,
		request: string(p.call.Arguments), response: out.text, err: err,
	})
	return contracts.ChatMessage{
		Role: contracts.ChatRoleTool, ToolCallID: p.call.ID, ToolError: err != nil,
		Parts: []contracts.ContentPart{{Text: out.text}},
	}
}
