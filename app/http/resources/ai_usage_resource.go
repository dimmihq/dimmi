/*
|--------------------------------------------------------------------------
| Agent Usage Resource
|--------------------------------------------------------------------------
|
| One month, for the settings overview: what the agent cost, how much
| it did, day by day in UTC, by model and by how runs ended.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/ai"
)

/*
AiUsage is one month's tallies. The counts come from the running tally
the cap reads; the series, the models and the outcomes from the
month's runs. MonthlySpendCapMicros is null when there is no cap.
*/
type AiUsage struct {
	PeriodStart           string           `json:"period_start"`
	CostMicros            int64            `json:"cost_micros"`
	InputTokens           int64            `json:"input_tokens"`
	CachedInputTokens     int64            `json:"cached_input_tokens"`
	OutputTokens          int64            `json:"output_tokens"`
	RunsCount             int              `json:"runs_count"`
	AnsweredCount         int              `json:"answered_count"`
	UnknownCount          int              `json:"unknown_count"`
	HandoffCount          int              `json:"handoff_count"`
	SilentCount           int              `json:"silent_count"`
	MonthlySpendCapMicros *int64           `json:"monthly_spend_cap_micros"`
	CappedAt              *time.Time       `json:"capped_at"`
	Daily                 []AiUsageDay     `json:"daily"`
	ByModel               []AiUsageModel   `json:"by_model"`
	ByOutcome             []AiUsageOutcome `json:"by_outcome"`
}

/*
AiUsageDay is one day's cost and runs.
*/
type AiUsageDay struct {
	Day        string `json:"day"`
	CostMicros int64  `json:"cost_micros"`
	RunsCount  int    `json:"runs_count"`
}

/*
AiUsageModel is one model's spend.
*/
type AiUsageModel struct {
	Model      string `json:"model"`
	CostMicros int64  `json:"cost_micros"`
	Calls      int    `json:"calls"`
}

/*
AiUsageOutcome is how runs ended.
*/
type AiUsageOutcome struct {
	Outcome   string `json:"outcome"`
	RunsCount int    `json:"runs_count"`
}

/*
NewAiUsage shapes the usage view.
*/
func NewAiUsage(v ai.UsageView) AiUsage {
	p := v.Period
	out := AiUsage{
		PeriodStart: p.PeriodStart.Time.Format("2006-01-02"), CostMicros: p.CostMicros,
		InputTokens: p.InputTokens, CachedInputTokens: p.CachedInputTokens, OutputTokens: p.OutputTokens,
		RunsCount: int(p.RunsCount), AnsweredCount: int(p.AnsweredCount), UnknownCount: int(p.UnknownCount),
		HandoffCount: int(p.HandoffCount), SilentCount: int(p.SilentCount),
		MonthlySpendCapMicros: v.CapMicros, CappedAt: p.CappedAt,
		Daily:     make([]AiUsageDay, len(v.Daily)),
		ByModel:   make([]AiUsageModel, len(v.ByModel)),
		ByOutcome: make([]AiUsageOutcome, len(v.ByOutcome)),
	}
	for i, d := range v.Daily {
		out.Daily[i] = AiUsageDay{
			Day: d.Day.Time.Format("2006-01-02"), CostMicros: d.CostMicros, RunsCount: int(d.RunsCount),
		}
	}
	for i, m := range v.ByModel {
		out.ByModel[i] = AiUsageModel{Model: m.Model, CostMicros: m.CostMicros, Calls: int(m.Calls)}
	}
	for i, o := range v.ByOutcome {
		out.ByOutcome[i] = AiUsageOutcome{Outcome: o.Outcome, RunsCount: int(o.RunsCount)}
	}
	return out
}
