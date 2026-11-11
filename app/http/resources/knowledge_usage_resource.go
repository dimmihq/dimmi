/*
|--------------------------------------------------------------------------
| Knowledge Usage Resource
|--------------------------------------------------------------------------
|
| A month's knowledge usage as the API shows it: per source, what was
| embedded and crawled and what it cost, in millionths of a dollar, so
| a dashboard or an invoice reads the same figures. Tokens the provider
| did not count are flagged as estimated, and usage without a price is
| flagged as unpriced rather than shown as free.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/knowledgesources"

/*
KnowledgeUsage is a month's usage: when it starts, its total cost and
its lines.
*/
type KnowledgeUsage struct {
	Object      string               `json:"object"`
	PeriodStart string               `json:"period_start"`
	CostMicros  int64                `json:"cost_micros"`
	Unpriced    bool                 `json:"unpriced"`
	Lines       []KnowledgeUsageLine `json:"lines"`
}

/*
KnowledgeUsageLine is one source's usage of one kind, the source null
for the workspace's own articles and conversations.
*/
type KnowledgeUsageLine struct {
	SourceID        *string `json:"source_id"`
	SourceName      string  `json:"source_name"`
	SourceType      string  `json:"source_type"`
	Kind            string  `json:"kind"`
	InputTokens     int64   `json:"input_tokens"`
	EstimatedTokens int64   `json:"estimated_tokens"`
	BrowserMs       int64   `json:"browser_ms"`
	Requests        int64   `json:"requests"`
	CostMicros      int64   `json:"cost_micros"`
	Unpriced        bool    `json:"unpriced"`
}

/*
NewKnowledgeUsage shapes a month's usage.
*/
func NewKnowledgeUsage(period string, lines []knowledgesources.UsageLine) KnowledgeUsage {
	out := KnowledgeUsage{Object: "knowledge_usage", PeriodStart: period, Lines: []KnowledgeUsageLine{}}

	for _, l := range lines {
		line := KnowledgeUsageLine{
			SourceName: l.SourceName, SourceType: l.SourceType, Kind: l.Kind, InputTokens: l.InputTokens,
			EstimatedTokens: l.EstimatedTokens, BrowserMs: l.BrowserMs, Requests: l.Requests,
			CostMicros: l.CostMicros, Unpriced: l.Unpriced,
		}
		if l.SourceID != nil {
			id := KnowledgeSourceID(*l.SourceID)
			line.SourceID = &id
		}

		out.CostMicros += l.CostMicros
		out.Unpriced = out.Unpriced || l.Unpriced
		out.Lines = append(out.Lines, line)
	}

	return out
}
