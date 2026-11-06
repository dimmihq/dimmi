/*
|--------------------------------------------------------------------------
| Agent Playground Resource
|--------------------------------------------------------------------------
|
| What the agent would do with a test question: how it read it, what it
| found, the answer and its citations, and what that took and cost.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
AiPlaygroundAnswer is one playground answer. RunID is the recorded
playground run, null when none was kept.
*/
type AiPlaygroundAnswer struct {
	Object         string               `json:"object"`
	RunID          *string              `json:"run_id"`
	Outcome        string               `json:"outcome"`
	Classification AiPlaygroundClass    `json:"classification"`
	Retrieved      []AiPlaygroundSource `json:"retrieved"`
	Answer         AiPlaygroundReply    `json:"answer"`
	Usage          AiPlaygroundUsage    `json:"usage"`
	Timing         AiPlaygroundTiming   `json:"timing"`
}

/*
AiPlaygroundClass is how the agent read the question.
*/
type AiPlaygroundClass struct {
	Intent   string `json:"intent"`
	Language string `json:"language"`
	Query    string `json:"query"`
}

/*
AiPlaygroundReply is the reply the agent would send and what it cites.
*/
type AiPlaygroundReply struct {
	Body      string                 `json:"body"`
	Citations []AiPlaygroundCitation `json:"citations"`
}

/*
AiPlaygroundUsage is what the question consumed and cost.
*/
type AiPlaygroundUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	CachedInputTokens int64 `json:"cached_input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	CostMicros        int64 `json:"cost_micros"`
}

/*
AiPlaygroundTiming is how long the answer took.
*/
type AiPlaygroundTiming struct {
	FirstTokenMs int `json:"first_token_ms"`
	LatencyMs    int `json:"latency_ms"`
}

/*
NewAiPlaygroundAnswer shapes a playground answer for the API.
*/
func NewAiPlaygroundAnswer(a contracts.PlaygroundAnswer) AiPlaygroundAnswer {
	out := AiPlaygroundAnswer{
		Object: "ai_playground_answer", RunID: publicPtr(models.PrefixAiRun, a.RunID), Outcome: a.Outcome,
		Classification: AiPlaygroundClass{
			Intent: a.Classification.Intent, Language: a.Classification.Language, Query: a.Classification.Query,
		},
		Retrieved: make([]AiPlaygroundSource, len(a.Retrieved)),
		Answer:    AiPlaygroundReply{Body: a.Answer, Citations: make([]AiPlaygroundCitation, len(a.Citations))},
		Usage: AiPlaygroundUsage{
			InputTokens: a.InputTokens, CachedInputTokens: a.CachedInputTokens,
			OutputTokens: a.OutputTokens, CostMicros: a.CostMicros,
		},
		Timing: AiPlaygroundTiming{FirstTokenMs: a.FirstTokenMs, LatencyMs: a.LatencyMs},
	}
	for i, s := range a.Retrieved {
		out.Retrieved[i] = AiPlaygroundSource{
			N: s.N, Title: s.Title, URL: s.URL, Similarity: s.Similarity, Used: s.Used, Text: s.Text,
		}
	}
	for i, c := range a.Citations {
		out.Answer.Citations[i] = AiPlaygroundCitation{N: c.N, Title: c.Title, URL: c.URL}
	}
	return out
}
