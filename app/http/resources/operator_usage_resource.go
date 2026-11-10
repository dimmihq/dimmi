/*
|--------------------------------------------------------------------------
| Operator Usage Resource
|--------------------------------------------------------------------------
|
| One month of Operator as the API shows it: totals, the daily series,
| why it ran, for whom, with which models, what became of its changes,
| and its latest runs. Money is in micro-dollars, like the agent's own
| usage, so the two add up exactly.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorUsage is one month of Operator. AgentCostMicros is the whole
agent's spend that month, dimmi's and Operator's, which the cap holds.
*/
type OperatorUsage struct {
	PeriodStart           string                    `json:"period_start"`
	CostMicros            int64                     `json:"cost_micros"`
	RunsCount             int64                     `json:"runs_count"`
	FailedCount           int64                     `json:"failed_count"`
	InputTokens           int64                     `json:"input_tokens"`
	OutputTokens          int64                     `json:"output_tokens"`
	AgentCostMicros       int64                     `json:"agent_cost_micros"`
	MonthlySpendCapMicros *int64                    `json:"monthly_spend_cap_micros"`
	Daily                 []AiUsageDay              `json:"daily"`
	ByPurpose             []OperatorUsagePurpose    `json:"by_purpose"`
	ByMember              []OperatorUsageMember     `json:"by_member"`
	ByModel               []AiUsageModel            `json:"by_model"`
	Changesets            []OperatorUsageChangesets `json:"changesets"`
	Recent                []OperatorUsageRun        `json:"recent"`
}

/*
OperatorUsagePurpose is the runs for one reason: ask, brief or triage.
*/
type OperatorUsagePurpose struct {
	Purpose    string `json:"purpose"`
	RunsCount  int64  `json:"runs_count"`
	CostMicros int64  `json:"cost_micros"`
}

/*
OperatorUsageMember is the runs for one teammate.
*/
type OperatorUsageMember struct {
	MemberID   string `json:"member_id"`
	Name       string `json:"name"`
	RunsCount  int64  `json:"runs_count"`
	CostMicros int64  `json:"cost_micros"`
}

/*
OperatorUsageChangesets is how many changes ended in one status.
*/
type OperatorUsageChangesets struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

/*
OperatorUsageRun is one of the latest runs: why it ran, for whom, on
which thread and conversation, how it ended and what it cost.
*/
type OperatorUsageRun struct {
	ID                 string    `json:"id"`
	Purpose            string    `json:"purpose"`
	Status             string    `json:"status"`
	MemberName         string    `json:"member_name"`
	ThreadTitle        string    `json:"thread_title"`
	ConversationID     *string   `json:"conversation_id"`
	ConversationNumber *int64    `json:"conversation_number"`
	CostMicros         int64     `json:"cost_micros"`
	LatencyMs          *int32    `json:"latency_ms"`
	CreatedAt          time.Time `json:"created_at"`
}

/*
NewOperatorUsage shapes the usage view.
*/
func NewOperatorUsage(v operatorchat.UsageView) OperatorUsage {
	t := v.Totals
	out := OperatorUsage{
		PeriodStart: v.Start.Format("2006-01-02"), CostMicros: t.CostMicros, RunsCount: t.RunsCount,
		FailedCount: t.FailedCount, InputTokens: t.InputTokens, OutputTokens: t.OutputTokens,
		AgentCostMicros: v.AgentCost, MonthlySpendCapMicros: v.CapMicros,
		Daily: make([]AiUsageDay, len(v.Daily)), ByPurpose: make([]OperatorUsagePurpose, len(v.ByPurpose)),
		ByMember: make([]OperatorUsageMember, len(v.ByMember)), ByModel: make([]AiUsageModel, len(v.ByModel)),
		Changesets: make([]OperatorUsageChangesets, len(v.Changesets)),
		Recent:     make([]OperatorUsageRun, len(v.Recent)),
	}
	for i, d := range v.Daily {
		out.Daily[i] = AiUsageDay{
			Day: d.Day.Time.Format("2006-01-02"), CostMicros: d.CostMicros, RunsCount: int(d.RunsCount),
		}
	}
	for i, p := range v.ByPurpose {
		out.ByPurpose[i] = OperatorUsagePurpose{Purpose: p.Purpose, RunsCount: p.RunsCount, CostMicros: p.CostMicros}
	}
	for i, m := range v.ByMember {
		out.ByMember[i] = OperatorUsageMember{
			MemberID: models.PublicID(models.PrefixMember, m.MemberID), Name: m.Name,
			RunsCount: m.RunsCount, CostMicros: m.CostMicros,
		}
	}
	for i, m := range v.ByModel {
		out.ByModel[i] = AiUsageModel{Model: m.Model, CostMicros: m.CostMicros, Calls: int(m.Calls)}
	}
	for i, c := range v.Changesets {
		out.Changesets[i] = OperatorUsageChangesets{Status: c.Status, Count: c.ChangesetsCount}
	}
	for i, r := range v.Recent {
		out.Recent[i] = newOperatorUsageRun(r)
	}
	return out
}

/*
newOperatorUsageRun shapes one of the latest runs.
*/
func newOperatorUsageRun(r models.ListRecentOperatorRunsRow) OperatorUsageRun {
	run := OperatorUsageRun{
		ID: models.PublicID(models.PrefixAiRun, r.ID), Purpose: r.Purpose, Status: r.Status,
		MemberName: r.MemberName, ThreadTitle: r.ThreadTitle, ConversationNumber: r.ConversationNumber,
		CostMicros: r.CostMicros, LatencyMs: r.LatencyMs, CreatedAt: r.CreatedAt,
	}
	if r.ConversationID != nil {
		id := models.PublicID(models.PrefixConversation, *r.ConversationID)
		run.ConversationID = &id
	}
	return run
}
