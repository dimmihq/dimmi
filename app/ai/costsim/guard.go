/*
|--------------------------------------------------------------------------
| Spend Guard
|--------------------------------------------------------------------------
|
| Every model call a simulation makes, by the agent or by the simulated
| customer, passes through the guard. Before a call leaves, the guard
| prices the worst case it could cost, the whole request as fresh input
| and the full output allowance, and refuses the call when that would
| take the run or the current conversation past its limit. After the
| call it adds what the call really cost, from the usage the provider
| reported and the same price table the app records costs with. A
| refusal is a provider error to the pipeline, which hands the
| conversation over as it would on any provider failure, and the
| harness then stops cleanly.
|
*/

package costsim

import (
	"context"
	"errors"
	"sync"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
ErrLimit is what a refused call fails with.
*/
var ErrLimit = &contracts.LLMError{Kind: contracts.LLMErrorQuota, Message: "the simulation's spend limit was reached"}

/*
Budget is the spend limits of a run and what it has spent, in micro
dollars. Zero for a limit means none.
*/
type Budget struct {
	mu           sync.Mutex
	prices       *aiprices.Table
	MaxRun       int64
	MaxPerConv   int64
	spent        int64
	conversation int64
	customer     int64
	calls        int
	refused      string
	worst        int64
	unpriced     map[string]bool
	pricing      sync.Mutex
}

/*
NewBudget returns a budget with the limits, priced from the table.
*/
func NewBudget(prices *aiprices.Table, maxRun, maxPerConv int64) *Budget {
	return &Budget{prices: prices, MaxRun: maxRun, MaxPerConv: maxPerConv, unpriced: map[string]bool{}}
}

/*
StartConversation resets the per-conversation spend.
*/
func (b *Budget) StartConversation() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.conversation = 0
}

/*
Spent returns the run's spend so far, the customer simulator's share
of it, and how many calls were made.
*/
func (b *Budget) Spent() (total, customer int64, calls int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spent, b.customer, b.calls
}

/*
Refused names the limit the guard last refused a call for, run or
conversation, and clears it; "" when it refused nothing.
*/
func (b *Budget) Refused() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	by := b.refused
	b.refused = ""
	return by
}

/*
Largest is the largest worst case of any call so far: the least a new
turn should be started with.
*/
func (b *Budget) Largest() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.worst
}

/*
Unpriced lists models the price table does not know, whose calls were
counted as free.
*/
func (b *Budget) Unpriced() []string {
	b.pricing.Lock()
	defer b.pricing.Unlock()
	var out []string
	for m := range b.unpriced {
		out = append(out, m)
	}
	return out
}

/*
Remaining returns what the run and the conversation may still spend,
-1 for no limit.
*/
func (b *Budget) Remaining() (run, conversation int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	run, conversation = -1, -1
	if b.MaxRun > 0 {
		run = b.MaxRun - b.spent
	}
	if b.MaxPerConv > 0 {
		conversation = b.MaxPerConv - b.conversation
	}
	return run, conversation
}

/*
WorstCase prices a request as if every input token were fresh and the
model wrote its whole output allowance.
*/
func (b *Budget) WorstCase(provider string, req contracts.ChatRequest) int64 {
	chars := 0
	for _, s := range req.System {
		chars += len(s.Text)
	}
	for _, m := range req.Messages {
		for _, p := range m.Parts {
			chars += len(p.Text)
		}
		for _, c := range m.ToolCalls {
			chars += len(c.Arguments) + len(c.Name)
		}
	}
	for _, t := range req.Tools {
		chars += len(t.Name) + len(t.Description) + len(t.Schema)
	}
	out := req.MaxOutputTokens
	if out <= 0 {
		out = 4096
	}
	return b.price(provider, req.Model, contracts.LLMUsage{InputTokens: int64(chars/3 + 64), OutputTokens: int64(out)})
}

/*
price returns a usage's cost, noting models the table does not know.
*/
func (b *Budget) price(provider, model string, u contracts.LLMUsage) int64 {
	if b.prices == nil {
		return 0
	}
	c := b.prices.Cost(provider, model, aiprices.TokenCounts{
		Input: u.InputTokens, CachedInput: u.CachedInputTokens, CacheWrite: u.CacheWriteTokens, Output: u.OutputTokens,
	})
	if c == nil {
		b.pricing.Lock()
		b.unpriced[provider+"/"+model] = true
		b.pricing.Unlock()
		return 0
	}
	return *c
}

/*
admit checks the worst case of a call against the limits and counts it.
*/
func (b *Budget) admit(provider string, req contracts.ChatRequest) error {
	worst := b.WorstCase(provider, req)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.worst = max(b.worst, worst)
	switch {
	case b.MaxRun > 0 && b.spent+worst > b.MaxRun:
		b.refused = "run"
		return ErrLimit
	case b.MaxPerConv > 0 && b.conversation+worst > b.MaxPerConv:
		b.refused = "conversation"
		return ErrLimit
	}
	b.calls++
	return nil
}

/*
charge adds what a finished call of the request cost.
*/
func (b *Budget) charge(provider string, req contracts.ChatRequest, u contracts.LLMUsage) {
	b.chargeMicros(b.price(provider, req.Model, u), req)
}

/*
chargeMicros adds a cost to the run and the conversation, and to the
customer simulator's share when the request was the customer's.
*/
func (b *Budget) chargeMicros(cost int64, req contracts.ChatRequest) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.spent += cost
	b.conversation += cost
	if req.Metadata[metaRole] == roleCustomer {
		b.customer += cost
	}
}

/*
Guarded wraps a driver so every call passes the budget. Calls the
simulated customer makes are marked in their metadata and reported
apart.
*/
type Guarded struct {
	contracts.LLM
	Budget   *Budget
	provider string
}

/*
Guard wraps the driver for a connection of the provider.
*/
func Guard(llm contracts.LLM, b *Budget, provider string) *Guarded {
	return &Guarded{LLM: llm, Budget: b, provider: provider}
}

/*
Chat runs a call the budget admits.
*/
func (g *Guarded) Chat(ctx context.Context, req contracts.ChatRequest) (*contracts.ChatResponse, error) {
	if err := g.Budget.admit(g.provider, req); err != nil {
		return nil, err
	}
	res, err := g.LLM.Chat(ctx, req)
	if res != nil {
		g.Budget.charge(g.provider, req, res.Usage)
	}
	return res, err
}

/*
Stream runs a call the budget admits and charges it when its usage
arrives.
*/
func (g *Guarded) Stream(ctx context.Context, req contracts.ChatRequest) (contracts.ChatStream, error) {
	if err := g.Budget.admit(g.provider, req); err != nil {
		return nil, err
	}
	s, err := g.LLM.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	return &guardedStream{ChatStream: s, g: g, model: req.Model, req: req}, nil
}

/*
guardedStream charges a stream's usage as it passes.
*/
type guardedStream struct {
	contracts.ChatStream
	g       *Guarded
	model   string
	req     contracts.ChatRequest
	written int
	charged bool
}

/*
Next passes each event on, charging the reported usage, or, for a
provider that reports none, an estimate from the request and the text
written, as the recorder estimates it.
*/
func (s *guardedStream) Next(ctx context.Context) (contracts.StreamEvent, error) {
	ev, err := s.ChatStream.Next(ctx)
	if err != nil || s.charged {
		return ev, err
	}
	switch {
	case ev.Kind == contracts.StreamEventTextDelta:
		s.written += len(ev.Text)
	case ev.Kind == contracts.StreamEventUsage && ev.Usage != nil:
		s.charged = true
		s.g.Budget.charge(s.g.provider, s.req, *ev.Usage)
	case ev.Kind == contracts.StreamEventDone:
		s.charged = true
		in := s.g.Budget.WorstCase(s.g.provider, contracts.ChatRequest{
			Model: s.model, System: s.req.System,
			Messages: s.req.Messages, Tools: s.req.Tools, MaxOutputTokens: 1,
		})
		s.g.Budget.chargeMicros(in+s.g.Budget.price(s.g.provider, s.model,
			contracts.LLMUsage{OutputTokens: int64(s.written/4 + 1)}), s.req)
	}
	return ev, err
}

/*
IsLimit reports whether an error is the guard's refusal.
*/
func IsLimit(err error) bool {
	var e *contracts.LLMError
	return errors.As(err, &e) && e == ErrLimit
}
