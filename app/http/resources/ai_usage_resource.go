/*
|--------------------------------------------------------------------------
| Agent Usage Resource
|--------------------------------------------------------------------------
|
| The month so far, for the settings overview: what the agent cost,
| how much it did, day by day, by model and by how runs ended.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
AiUsage is the month's tallies.
*/
type AiUsage struct {
	PeriodStart   string           `json:"period_start"`
	CostMicros    int64            `json:"cost_micros"`
	InputTokens   int64            `json:"input_tokens"`
	OutputTokens  int64            `json:"output_tokens"`
	RunsCount     int              `json:"runs_count"`
	AnsweredCount int              `json:"answered_count"`
	UnknownCount  int              `json:"unknown_count"`
	HandoffCount  int              `json:"handoff_count"`
	CappedAt      *time.Time       `json:"capped_at"`
	Daily         []AiUsageDay     `json:"daily"`
	ByModel       []AiUsageModel   `json:"by_model"`
	ByOutcome     []AiUsageOutcome `json:"by_outcome"`
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
func NewAiUsage(period models.AiSpendPeriod, daily []models.ListDailySpendRow,
	byModel []models.ListSpendByModelRow, byOutcome []models.ListSpendByOutcomeRow,
) AiUsage {
	out := AiUsage{
		PeriodStart:  period.PeriodStart.Time.Format("2006-01-02"),
		CostMicros:   period.CostMicros,
		InputTokens:  period.InputTokens,
		OutputTokens: period.OutputTokens,
		RunsCount:    int(period.RunsCount),
		CappedAt:     period.CappedAt,
	}
	out.Daily = make([]AiUsageDay, len(daily))
	for i, d := range daily {
		out.Daily[i] = AiUsageDay{
			Day: d.Day.Time.Format("2006-01-02"), CostMicros: d.CostMicros, RunsCount: int(d.RunsCount),
		}
	}
	out.ByModel = make([]AiUsageModel, len(byModel))
	for i, m := range byModel {
		out.ByModel[i] = AiUsageModel{Model: m.Model, CostMicros: m.CostMicros, Calls: int(m.Calls)}
	}
	out.ByOutcome = make([]AiUsageOutcome, len(byOutcome))
	for i, o := range byOutcome {
		out.ByOutcome[i] = AiUsageOutcome{Outcome: o.Outcome, RunsCount: int(o.RunsCount)}
		switch o.Outcome {
		case "answered":
			out.AnsweredCount += int(o.RunsCount)
		case "unknown":
			out.UnknownCount += int(o.RunsCount)
		case "handoff":
			out.HandoffCount += int(o.RunsCount)
		}
	}
	return out
}
