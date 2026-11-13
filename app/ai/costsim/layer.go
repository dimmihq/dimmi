/*
|--------------------------------------------------------------------------
| Tool Layer Figures
|--------------------------------------------------------------------------
|
| With the tool layer on, each run keeps what the layer did. The harness
| reads it back per conversation: tools loaded and reloaded, call_tool
| refusals, names the model asked for that do not exist, whether the
| tools the model ran were among those the newest message suggested, and
| for every model call how much of its input came from the cache, so the
| report shows the share at the first call of a turn, right after the
| history was pruned, beside the calls after it.
|
*/

package costsim

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/toolset"
)

/*
LayerResult is what the layer did in one conversation or request.
*/
type LayerResult struct {
	Runs         int         `json:"runs"`
	Loads        int         `json:"loads"`
	Reloads      int         `json:"reloads"`
	Calls        int         `json:"calls"`
	ArgErrors    int         `json:"argument_errors"`
	NotFound     []string    `json:"not_found,omitempty"`
	Used         int         `json:"used"`
	UsedHits     int         `json:"used_in_suggestions"`
	Missed       []string    `json:"used_not_suggested,omitempty"`
	FixedTokens  int         `json:"fixed_tokens"`
	GroupsTokens int         `json:"groups_tokens"`
	SystemTokens int         `json:"system_tokens"`
	Steps        []CallCache `json:"calls_cache"`
}

/*
CallCache is one model call's input: its turn, its place in the turn,
and its fresh and cached tokens.
*/
type CallCache struct {
	Turn   int   `json:"turn"`
	Call   int   `json:"call"`
	Fresh  int64 `json:"fresh"`
	Cached int64 `json:"cached"`
}

/*
readLayer reads the layer's records and the calls' cache use of the
runs, oldest first.
*/
func (h *Harness) readLayer(ctx context.Context, ws ids.UUID, runs []ids.UUID) (*LayerResult, error) {
	if len(runs) == 0 {
		return nil, nil
	}
	out := &LayerResult{}
	rows, err := h.App.DB.Query(ctx, `SELECT tool_layer FROM ai_runs
		WHERE workspace_id = $1 AND id = ANY($2) AND tool_layer IS NOT NULL ORDER BY created_at`, ws, runs)
	if err != nil {
		return nil, fmt.Errorf("read layer records: %w", err)
	}
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		var r toolset.Record
		if json.Unmarshal(raw, &r) == nil {
			out.add(r)
		}
	}
	rows.Close()
	if out.Runs == 0 {
		return nil, rows.Err()
	}
	return out, h.readCalls(ctx, ws, runs, out)
}

/*
add folds one run's record in.
*/
func (l *LayerResult) add(r toolset.Record) {
	l.Runs++
	l.Loads += r.Stats.Loads
	l.Reloads += r.Stats.Reloads
	l.Calls += r.Stats.Calls
	l.ArgErrors += r.Stats.ArgErrors
	l.NotFound = append(l.NotFound, r.Stats.NotFound...)
	for _, u := range r.Stats.Used {
		l.Used++
		if slices.Contains(r.Suggested, u) {
			l.UsedHits++
		} else {
			l.Missed = append(l.Missed, u)
		}
	}
	l.FixedTokens = max(l.FixedTokens, r.FixedTokens)
	l.GroupsTokens = max(l.GroupsTokens, r.GroupsTokens)
	l.SystemTokens = max(l.SystemTokens, r.SystemTokens)
}

/*
readCalls reads each model call's fresh and cached input, numbering
turns by run and calls within each run.
*/
func (h *Harness) readCalls(ctx context.Context, ws ids.UUID, runs []ids.UUID, out *LayerResult) error {
	rows, err := h.App.DB.Query(ctx, `SELECT s.run_id, s.input_tokens, s.cached_input_tokens
		FROM ai_run_steps s JOIN ai_runs r ON r.id = s.run_id
		WHERE r.workspace_id = $1 AND r.id = ANY($2) AND s.kind = 'generate'
		ORDER BY r.created_at, s.seq`, ws, runs)
	if err != nil {
		return fmt.Errorf("read call cache: %w", err)
	}
	defer rows.Close()
	turn, call := 0, 0
	var last ids.UUID
	for rows.Next() {
		var run ids.UUID
		var c CallCache
		if err := rows.Scan(&run, &c.Fresh, &c.Cached); err != nil {
			return err
		}
		if run != last {
			turn, call, last = turn+1, 0, run
		}
		call++
		c.Turn, c.Call = turn, call
		out.Steps = append(out.Steps, c)
	}
	return rows.Err()
}

/*
runIDs lists the ids of the runs a query selects.
*/
func (h *Harness) runIDs(ctx context.Context, query string, args ...any) ([]ids.UUID, error) {
	rows, err := h.App.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ids.UUID
	for rows.Next() {
		var id ids.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

/*
LeftOut is a catalog tool out of Operator's reach, and why.
*/
type LeftOut struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

/*
layerMarkdown is the report's section on the tool layer, empty when no
run used it.
*/
func (r *Report) layerMarkdown() string {
	byKind := r.layersByKind()
	if len(byKind) == 0 && len(r.LeftOut) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Tool layer\n\n| slice | n | fixed tools tok | groups tok | system tok | loads/conv | " +
		"reloads/conv | calls/conv | arg errors | not found | suggested hit |\n" + strings.Repeat("|---", 11) + "|\n")
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
	for _, k := range kinds {
		writeLayerRow(&b, k, byKind[k])
	}
	writeCacheShares(&b, kinds, byKind)
	writeMisses(&b, kinds, byKind)
	writeLeftOut(&b, r.LeftOut)
	return b.String()
}

/*
layersByKind groups the conversations' layer figures by customer and
Operator.
*/
func (r *Report) layersByKind() map[string][]*LayerResult {
	out := map[string][]*LayerResult{}
	for _, c := range r.Conversations {
		if c.Layer == nil {
			continue
		}
		kind := "customer"
		if c.Kind == "operator" {
			kind = "operator"
		}
		out[kind] = append(out[kind], c.Layer)
	}
	return out
}

/*
writeCacheShares writes the cached share of input at each call of a
turn, for the first turn and for the turns after it.
*/
func writeCacheShares(b *strings.Builder, kinds []string, byKind map[string][]*LayerResult) {
	b.WriteString("\n### Cached share of input by call\n\n| slice | call | first turn | later turns |\n" +
		"|---|---|---|---|\n")
	for _, k := range kinds {
		for call := 1; call <= 4; call++ {
			first := cacheShare(byKind[k], call, func(turn int) bool { return turn == 1 })
			later := cacheShare(byKind[k], call, func(turn int) bool { return turn > 1 })
			if first == "" && later == "" {
				continue
			}
			label := strconv.Itoa(call)
			if call == 4 {
				label = "4+"
			}
			fmt.Fprintf(b, "| %s | %s | %s | %s |\n", k, label, first, later)
		}
	}
}

/*
writeLeftOut lists the catalog tools out of Operator's reach by reason.
*/
func writeLeftOut(b *strings.Builder, left []LeftOut) {
	if len(left) == 0 {
		return
	}
	reasons := map[string][]string{}
	for _, l := range left {
		reasons[l.Reason] = append(reasons[l.Reason], l.Name)
	}
	b.WriteString("\n### Catalog tools out of Operator's reach\n\n")
	for reason, names := range reasons {
		fmt.Fprintf(b, "- %d tools: %s. %s\n", len(names), reason, strings.Join(names, ", "))
	}
}

/*
writeLayerRow writes one slice's row.
*/
func writeLayerRow(b *strings.Builder, kind string, ls []*LayerResult) {
	var fixed, groups, system, loads, reloads, calls, errs, notFound, used, hits int
	for _, l := range ls {
		fixed, groups, system = max(fixed, l.FixedTokens), max(groups, l.GroupsTokens), max(system, l.SystemTokens)
		loads, reloads, calls, errs = loads+l.Loads, reloads+l.Reloads, calls+l.Calls, errs+l.ArgErrors
		notFound, used, hits = notFound+len(l.NotFound), used+l.Used, hits+l.UsedHits
	}
	n := float64(len(ls))
	hit := "-"
	if used > 0 {
		hit = fmt.Sprintf("%d/%d (%.0f%%)", hits, used, float64(hits)/float64(used)*100)
	}
	fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %.1f | %.1f | %.1f | %d | %d | %s |\n", kind, len(ls), fixed, groups,
		system, float64(loads)/n, float64(reloads)/n, float64(calls)/n, errs, notFound, hit)
}

/*
cacheShare is the cached share of input at a call position, over the
first turn or the later ones; "" when there are no such calls.
*/
func cacheShare(ls []*LayerResult, call int, turns func(int) bool) string {
	var fresh, cached int64
	n := 0
	for _, l := range ls {
		for _, s := range l.Steps {
			if !turns(s.Turn) || (call < 4 && s.Call != call) || (call == 4 && s.Call < 4) {
				continue
			}
			fresh, cached, n = fresh+s.Fresh, cached+s.Cached, n+1
		}
	}
	if n == 0 || fresh+cached == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%% of %d tok (n=%d)", float64(cached)/float64(fresh+cached)*100, (fresh+cached)/int64(n), n)
}

/*
writeMisses lists names the model could not find and tools it ran that
were not suggested.
*/
func writeMisses(b *strings.Builder, kinds []string, byKind map[string][]*LayerResult) {
	for _, k := range kinds {
		var notFound, missed []string
		for _, l := range byKind[k] {
			notFound, missed = append(notFound, l.NotFound...), append(missed, l.Missed...)
		}
		if len(notFound) > 0 {
			fmt.Fprintf(b, "\n- %s, names not found: %s\n", k, strings.Join(notFound, ", "))
		}
		if len(missed) > 0 {
			fmt.Fprintf(b, "- %s, ran but not suggested: %s\n", k, strings.Join(missed, ", "))
		}
	}
}
