/*
|--------------------------------------------------------------------------
| Simulation Report
|--------------------------------------------------------------------------
|
| What a run measured, per conversation and overall: the average,
| median, 90th, 95th and 99th percentile and maximum cost per
| conversation and per AI reply, how many conversations cost more than
| 1, 2, 5 and 10 cents, how much input came from the prompt cache, and
| the same figures for each kind of scenario and each model. It is
| written as JSON for tools and as a short summary for people.
|
*/

package costsim

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/filesystem"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
Stats summarizes a list of costs in micro dollars.
*/
type Stats struct {
	Count  int     `json:"count"`
	Mean   float64 `json:"mean_micros"`
	Median int64   `json:"median_micros"`
	P90    int64   `json:"p90_micros"`
	P95    int64   `json:"p95_micros"`
	P99    int64   `json:"p99_micros"`
	Max    int64   `json:"max_micros"`
	Total  int64   `json:"total_micros"`
}

/*
StatsOf summarizes the costs.
*/
func StatsOf(costs []int64) Stats {
	if len(costs) == 0 {
		return Stats{}
	}
	sorted := slices.Clone(costs)
	slices.Sort(sorted)
	var total int64
	for _, c := range sorted {
		total += c
	}
	at := func(p float64) int64 {
		i := int(p*float64(len(sorted))+0.999999) - 1
		return sorted[min(max(i, 0), len(sorted)-1)]
	}
	return Stats{
		Count: len(sorted), Mean: float64(total) / float64(len(sorted)), Median: at(0.5), P90: at(0.9),
		P95: at(0.95), P99: at(0.99), Max: sorted[len(sorted)-1], Total: total,
	}
}

/*
Group is the figures for one slice of the conversations.
*/
type Group struct {
	PerConversation Stats              `json:"per_conversation"`
	PerReply        Stats              `json:"per_ai_reply"`
	Above           map[string]float64 `json:"share_above"`
	CacheHitShare   float64            `json:"cache_hit_share"`
	AvgTurns        float64            `json:"avg_turns"`
	AvgReplies      float64            `json:"avg_ai_replies"`
	AvgModelCalls   float64            `json:"avg_model_calls"`
	AvgSearches     float64            `json:"avg_searches"`
	AvgToolCalls    float64            `json:"avg_tool_calls"`
	Outcomes        map[string]int     `json:"outcomes"`
}

/*
Report is a whole run.
*/
type Report struct {
	Started             time.Time            `json:"started"`
	Finished            time.Time            `json:"finished"`
	Options             map[string]any       `json:"options"`
	AgentModel          string               `json:"agent_model"`
	StoppedBy           string               `json:"stopped_by,omitempty"`
	SpendMicros         int64                `json:"guard_spend_micros"`
	CustomerSpendMicros int64                `json:"customer_spend_micros"`
	Calls               int                  `json:"guarded_calls"`
	Unpriced            []string             `json:"unpriced_models,omitempty"`
	Overall             Group                `json:"overall"`
	ByKind              map[string]Group     `json:"by_kind"`
	ByModel             map[string]Group     `json:"by_model"`
	Conversations       []ConversationResult `json:"conversations"`
}

/*
thresholds are the costs, in micro dollars, the report counts
conversations above.
*/
func thresholds() []threshold {
	return []threshold{{"1c", 10_000}, {"2c", 20_000}, {"5c", 50_000}, {"10c", 100_000}}
}

/*
threshold is a cost the report counts conversations above.
*/
type threshold struct {
	name   string
	micros int64
}

/*
groupOf works out a group's figures.
*/
func groupOf(convs []ConversationResult) Group {
	g := Group{Above: map[string]float64{}, Outcomes: map[string]int{}}
	if len(convs) == 0 {
		return g
	}
	var per, replies []int64
	var fresh, cached int64
	var turns, ai, calls, searches, tools int
	for _, c := range convs {
		per = append(per, c.CostMicros)
		replies = append(replies, c.ReplyCosts...)
		fresh, cached = fresh+c.NewTokens, cached+c.CachedTokens
		turns, ai, calls = turns+c.Turns, ai+c.AIReplies, calls+c.ModelCalls
		searches, tools = searches+c.Searches, tools+c.ToolCalls
		g.Outcomes[c.Outcome]++
		for _, t := range thresholds() {
			if c.CostMicros > t.micros {
				g.Above[t.name]++
			}
		}
	}
	n := float64(len(convs))
	for _, t := range thresholds() {
		g.Above[t.name] /= n
	}
	g.PerConversation, g.PerReply = StatsOf(per), StatsOf(replies)
	if fresh+cached > 0 {
		g.CacheHitShare = float64(cached) / float64(fresh+cached)
	}
	g.AvgTurns, g.AvgReplies, g.AvgModelCalls = float64(turns)/n, float64(ai)/n, float64(calls)/n
	g.AvgSearches, g.AvgToolCalls = float64(searches)/n, float64(tools)/n
	return g
}

/*
summarize fills the overall figures and the splits.
*/
func (r *Report) summarize() {
	r.Overall = groupOf(r.Conversations)
	r.ByKind, r.ByModel = map[string]Group{}, map[string]Group{}
	kinds, byModel := map[string][]ConversationResult{}, map[string][]ConversationResult{}
	for _, c := range r.Conversations {
		kinds[c.Kind] = append(kinds[c.Kind], c)
		top, most := "none", 0
		for m, n := range c.Models {
			if n > most || (n == most && m < top) {
				top, most = m, n
			}
		}
		byModel[top] = append(byModel[top], c)
	}
	for k, list := range kinds {
		r.ByKind[k] = groupOf(list)
	}
	for m, list := range byModel {
		r.ByModel[m] = groupOf(list)
	}
}

/*
summary is the options as the report shows them.
*/
func (o Options) summary() map[string]any {
	return map[string]any{
		"conversations": o.Conversations, "mode": o.Mode, "agent_model": o.AgentModel,
		"classifier_model": o.ClassifierModel, "customer_model": o.CustomerModel,
		"max_spend_usd": float64(o.MaxSpend) / 1e6, "max_conversation_usd": float64(o.MaxPerConv) / 1e6,
		"max_turns": o.MaxTurns, "dry_run": o.DryRun, "scenarios": len(o.Scenarios),
	}
}

/*
Write saves the report as report.json and summary.md in the directory
of the disk and returns their paths on it.
*/
func (r *Report) Write(ctx context.Context, disk filesystem.Filesystem, dir string) (string, string, error) {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", "", err
	}
	js, md := path.Join(dir, "report.json"), path.Join(dir, "summary.md")
	if err := disk.Put(ctx, js, raw, filesystem.PutOptions{ContentType: "application/json"}); err != nil {
		return "", "", fmt.Errorf("write report: %w", err)
	}
	if err := disk.Put(ctx, md, []byte(r.Markdown()), filesystem.PutOptions{ContentType: "text/markdown"}); err != nil {
		return "", "", fmt.Errorf("write summary: %w", err)
	}
	return js, md, nil
}

/*
cents shows micro dollars as cents.
*/
func cents(micros float64) string {
	return fmt.Sprintf("%.3f¢", micros/1e4)
}

/*
Markdown is the short readable summary.
*/
func (r *Report) Markdown() string {
	var b strings.Builder
	dry := ""
	if r.Options["dry_run"] == true {
		dry = " (dry run: offline model, usage counted from text size)"
	}
	fmt.Fprintf(&b, "# AI cost simulation%s\n\n", dry)
	fmt.Fprintf(&b, "- %d conversations, agent model %s, customer %s\n", len(r.Conversations), r.AgentModel,
		r.Options["mode"])
	fmt.Fprintf(&b, "- spend guarded: $%.4f (customer simulator $%.4f) over %d model calls\n",
		float64(r.SpendMicros)/1e6, float64(r.CustomerSpendMicros)/1e6, r.Calls)
	if r.StoppedBy != "" {
		fmt.Fprintf(&b, "- stopped early: %s\n", r.StoppedBy)
	}
	if len(r.Unpriced) > 0 {
		fmt.Fprintf(&b, "- models without a price, counted as free: %s\n", strings.Join(r.Unpriced, ", "))
	}
	b.WriteString("\n## Overall\n\n")
	b.WriteString(groupTable([]string{"all"}, map[string]Group{"all": r.Overall}))
	b.WriteString("\n## By scenario kind\n\n")
	b.WriteString(groupTable(sortedKeys(r.ByKind), r.ByKind))
	b.WriteString("\n## By model\n\n")
	b.WriteString(groupTable(sortedKeys(r.ByModel), r.ByModel))
	b.WriteString("\n## Conversations\n\n| # | scenario | kind | turns | AI replies | calls | searches | tools | " +
		"new | cached | out | cost | outcome |\n" + strings.Repeat("|---", 13) + "|\n")
	for _, c := range r.Conversations {
		fmt.Fprintf(&b, "| %d | %s | %s | %d | %d | %d | %d | %d | %d | %d | %d | %s | %s |\n", c.Index, c.Scenario,
			c.Kind, c.Turns, c.AIReplies, c.ModelCalls, c.Searches, c.ToolCalls, c.NewTokens, c.CachedTokens,
			c.OutputTokens, cents(float64(c.CostMicros)), c.Outcome)
	}
	return b.String()
}

func groupTable(order []string, groups map[string]Group) string {
	var b strings.Builder
	b.WriteString("| slice | n | conv mean | median | p90 | p95 | p99 | max | reply mean | reply p95 | " +
		">1¢ | >2¢ | >5¢ | >10¢ | cache hit | turns | replies | calls |\n" + strings.Repeat("|---", 18) + "|\n")
	for _, k := range order {
		g := groups[k]
		c, rp := g.PerConversation, g.PerReply
		fmt.Fprintf(&b, "| %s | %d | %s | %s | %s | %s | %s | %s | %s | %s | "+
			"%.0f%% | %.0f%% | %.0f%% | %.0f%% | %.0f%% | %.1f | %.1f | %.1f |\n",
			k, c.Count, cents(c.Mean), cents(float64(c.Median)), cents(float64(c.P90)), cents(float64(c.P95)),
			cents(float64(c.P99)), cents(float64(c.Max)), cents(rp.Mean), cents(float64(rp.P95)),
			g.Above["1c"]*100, g.Above["2c"]*100, g.Above["5c"]*100, g.Above["10c"]*100, g.CacheHitShare*100,
			g.AvgTurns, g.AvgReplies, g.AvgModelCalls)
	}
	return b.String()
}

func sortedKeys(m map[string]Group) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

/*
Estimate is a rough cost for a run before it starts, in micro dollars.
Each customer turn is three calls on the agent's model: the first
carries the agent's instructions and tools, about twelve thousand
tokens, fresh on a conversation's first turn and mostly cached after;
the second adds the search results; the third is a short check. The
prompt grows with the transcript. A model-played customer adds one
small call a turn on its own model.
*/
func Estimate(b *Budget, provider string, o Options) int64 {
	var total int64
	for i := range o.Conversations {
		total += estimateConversation(b, provider, o, o.Scenarios[i%len(o.Scenarios)])
	}
	return total
}

/*
estimateConversation is the rough cost of one scenario's turns.
*/
func estimateConversation(b *Budget, provider string, o Options, sc Scenario) int64 {
	turns := len(sc.Messages)
	if o.Mode == ModeModel {
		turns = max(turns, 4)
	}
	if o.MaxTurns > 0 {
		turns = min(turns, o.MaxTurns)
	}
	var total int64
	for t := range turns {
		grown := int64(400*t + len(sc.Messages[min(t, len(sc.Messages)-1)])/4)
		first := usageOf(2000+grown, 10_000, 60)
		if t == 0 {
			first = usageOf(12_000+grown, 0, 60)
		}
		total += b.price(provider, o.AgentModel, first) +
			b.price(provider, o.AgentModel, usageOf(2500+grown, 12_000, 220)) +
			b.price(provider, o.AgentModel, usageOf(1500+grown, 0, 80))
		if o.Mode == ModeModel && t > 0 {
			total += b.price(provider, o.CustomerModel, usageOf(int64(500+300*t), 0, 60))
		}
	}
	return total
}

func usageOf(fresh, cached, out int64) contracts.LLMUsage {
	return contracts.LLMUsage{InputTokens: fresh, CachedInputTokens: cached, OutputTokens: out}
}
