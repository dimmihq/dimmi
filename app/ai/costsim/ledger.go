/*
|--------------------------------------------------------------------------
| Token Ledger Figures
|--------------------------------------------------------------------------
|
| Every model call keeps what its input was made of. The harness splits
| each call's reported tokens across those parts, prices each part with
| the call's own rates, and sums them per conversation or request, so the
| report shows where the tokens and the money went: the fixed prefix,
| history, the newest message, loaded tool definitions, search results,
| other tool results, output and thinking.
|
*/

package costsim

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/ledger"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
Parts of the ledger beside the input's.
*/
const (
	partOutput   = "output"
	partThinking = "thinking"
	partUnsplit  = "unsplit"
)

/*
PartCost is one part's tokens by how they were billed, and its cost.
*/
type PartCost struct {
	Cached  int64 `json:"cached"`
	Written int64 `json:"written"`
	Fresh   int64 `json:"fresh"`
	Micros  int64 `json:"micros"`
}

/*
Tokens is all of a part's tokens.
*/
func (p PartCost) Tokens() int64 {
	return p.Cached + p.Written + p.Fresh
}

/*
add sums another part's figures in.
*/
func (p PartCost) add(o PartCost) PartCost {
	return PartCost{p.Cached + o.Cached, p.Written + o.Written, p.Fresh + o.Fresh, p.Micros + o.Micros}
}

/*
bigPromptTokens is the prompt size above which some providers charge
more for the whole call.
*/
const bigPromptTokens = 100_000

/*
readLedgerAndBig sums the model calls of the runs by part and counts
the calls whose prompt passed 100,000 tokens.
*/
func (h *Harness) readLedgerAndBig(ctx context.Context, ws ids.UUID, runs []ids.UUID) (map[string]PartCost, int,
	error,
) {
	if len(runs) == 0 {
		return nil, 0, nil
	}
	rows, err := h.App.DB.Query(ctx, `SELECT coalesce(s.provider, ''), coalesce(s.model, ''), s.input_tokens,
			s.cached_input_tokens, s.cache_write_tokens, s.output_tokens, s.reasoning_tokens, s.ledger
		FROM ai_run_steps s JOIN ai_runs r ON r.id = s.run_id
		WHERE r.workspace_id = $1 AND r.id = ANY($2) AND s.kind = 'generate'`, ws, runs)
	if err != nil {
		return nil, 0, fmt.Errorf("read ledger: %w", err)
	}
	defer rows.Close()
	out := map[string]PartCost{}
	big := 0
	for rows.Next() {
		var provider, model string
		var u contracts.LLMUsage
		var raw json.RawMessage
		if err := rows.Scan(&provider, &model, &u.InputTokens, &u.CachedInputTokens, &u.CacheWriteTokens,
			&u.OutputTokens, &u.ReasoningTokens, &raw); err != nil {
			return nil, 0, err
		}
		if u.InputTokens+u.CachedInputTokens+u.CacheWriteTokens > bigPromptTokens {
			big++
		}
		for part, c := range h.priceCall(provider, model, u, raw) {
			out[part] = out[part].add(c)
		}
	}
	return out, big, rows.Err()
}

/*
priceCall splits one call by part and prices each part.
*/
func (h *Harness) priceCall(provider, model string, u contracts.LLMUsage, raw json.RawMessage) map[string]PartCost {
	var segments []ledger.Segment
	_ = json.Unmarshal(raw, &segments)
	split := ledger.Split(segments, u)
	if len(split) == 0 {
		split = map[string]ledger.Tokens{partUnsplit: {
			Cached: u.CachedInputTokens, Written: u.CacheWriteTokens, Fresh: u.InputTokens,
		}}
	}
	out := map[string]PartCost{}
	for part, t := range split {
		out[part] = PartCost{
			Cached: t.Cached, Written: t.Written, Fresh: t.Fresh,
			Micros: h.cost(provider, model, aiprices.TokenCounts{
				Input: t.Fresh, CachedInput: t.Cached, CacheWrite: t.Written,
			}),
		}
	}
	thinking := min(u.ReasoningTokens, u.OutputTokens)
	answer := u.OutputTokens - thinking
	out[partOutput] = PartCost{Fresh: answer, Micros: h.cost(provider, model, aiprices.TokenCounts{Output: answer})}
	if thinking > 0 {
		out[partThinking] = PartCost{
			Fresh: thinking, Micros: h.cost(provider, model, aiprices.TokenCounts{Output: thinking}),
		}
	}
	return out
}

/*
cost prices token counts, zero when the model has no price.
*/
func (h *Harness) cost(provider, model string, c aiprices.TokenCounts) int64 {
	if h.App.AI.Prices == nil {
		return 0
	}
	if v := h.App.AI.Prices.Cost(provider, model, c); v != nil {
		return *v
	}
	return 0
}

/*
ledgerOrder is the order parts are reported in.
*/
func ledgerOrder() []string {
	return []string{
		ledger.Fixed, ledger.History, ledger.Newest, ledger.Calls, ledger.Loaded, ledger.Search, ledger.Results,
		partUnsplit, partOutput, partThinking,
	}
}

/*
ledgerSlice is one slice's summed parts, conversations and replies.
*/
type ledgerSlice struct {
	parts   map[string]PartCost
	n       int
	replies int
}

/*
ledgerSlices sums the conversations' ledgers by customer and Operator.
*/
func (r *Report) ledgerSlices() map[string]*ledgerSlice {
	out := map[string]*ledgerSlice{}
	for _, c := range r.Conversations {
		if c.Ledger == nil {
			continue
		}
		kind := "customer"
		if c.Kind == "operator" {
			kind = "operator"
		}
		s := out[kind]
		if s == nil {
			s = &ledgerSlice{parts: map[string]PartCost{}}
			out[kind] = s
		}
		s.n++
		s.replies += c.AIReplies
		for p, v := range c.Ledger {
			s.parts[p] = s.parts[p].add(v)
		}
	}
	return out
}

/*
ledgerMarkdown is the report's token ledger: per slice, each part's
average tokens and cost per conversation and per AI reply, and its share
of the cost.
*/
func (r *Report) ledgerMarkdown() string {
	bySlice := r.ledgerSlices()
	if len(bySlice) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Token ledger\n\nAverage per conversation (Operator: per request). Input tokens are split " +
		"across parts by their share of the call's characters, cached tokens filling the parts from the start.\n\n" +
		"| slice | part | cached | written | fresh | tokens | cost | per reply | share |\n" +
		strings.Repeat("|---", 9) + "|\n")
	kinds := make([]string, 0, len(bySlice))
	for k := range bySlice {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
	for _, k := range kinds {
		writeLedgerRows(&b, k, bySlice[k])
	}
	return b.String()
}

/*
writeLedgerRows writes one slice's parts.
*/
func writeLedgerRows(b *strings.Builder, kind string, s *ledgerSlice) {
	var total int64
	for _, v := range s.parts {
		total += v.Micros
	}
	n, replies := float64(s.n), float64(max(s.replies, 1))
	for _, p := range ledgerOrder() {
		v, ok := s.parts[p]
		if !ok {
			continue
		}
		share := 0.0
		if total > 0 {
			share = float64(v.Micros) / float64(total) * 100
		}
		fmt.Fprintf(b, "| %s | %s | %.0f | %.0f | %.0f | %.0f | %s | %s | %.0f%% |\n", kind, p, float64(v.Cached)/n,
			float64(v.Written)/n, float64(v.Fresh)/n, float64(v.Tokens())/n, cents(float64(v.Micros)/n),
			cents(float64(v.Micros)/replies), share)
	}
}
