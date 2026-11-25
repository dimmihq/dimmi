/*
|--------------------------------------------------------------------------
| Knowledge Usage Resource
|--------------------------------------------------------------------------
|
| A month's knowledge usage as the API shows it: per source, what was
| embedded and crawled. Tokens the provider did not count are flagged
| as estimated.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/knowledgesources"

/*
KnowledgeUsage is a month's usage: when it starts, and its lines.
*/
type KnowledgeUsage struct {
	Object      string               `json:"object"`
	PeriodStart string               `json:"period_start"`
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
		}
		if l.SourceID != nil {
			id := KnowledgeSourceID(*l.SourceID)
			line.SourceID = &id
		}

		out.Lines = append(out.Lines, line)
	}

	return out
}
