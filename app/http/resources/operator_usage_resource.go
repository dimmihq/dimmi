/*
|--------------------------------------------------------------------------
| Operator Usage Resource
|--------------------------------------------------------------------------
|
| One month of Operator as the API shows it: totals, the daily series,
| why it ran, for whom, with which models, what became of its changes,
| and its latest runs.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/operatorchat"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorUsage is one month of Operator.
*/
type OperatorUsage struct {
	PeriodStart  string                    `json:"period_start"`
	RunsCount    int64                     `json:"runs_count"`
	FailedCount  int64                     `json:"failed_count"`
	InputTokens  int64                     `json:"input_tokens"`
	OutputTokens int64                     `json:"output_tokens"`
	Daily        []AiUsageDay              `json:"daily"`
	ByPurpose    []OperatorUsagePurpose    `json:"by_purpose"`
	ByMember     []OperatorUsageMember     `json:"by_member"`
	ByModel      []AiUsageModel            `json:"by_model"`
	Changesets   []OperatorUsageChangesets `json:"changesets"`
	Recent       []OperatorUsageRun        `json:"recent"`
	ByRoutine    []OperatorUsageRoutine    `json:"by_routine"`
}

/*
OperatorUsagePurpose is the runs for one reason: ask, brief, triage or
routine.
*/
type OperatorUsagePurpose struct {
	Purpose   string `json:"purpose"`
	RunsCount int64  `json:"runs_count"`
}

/*
OperatorUsageMember is the runs for one teammate.
*/
type OperatorUsageMember struct {
	MemberID  string `json:"member_id"`
	Name      string `json:"name"`
	RunsCount int64  `json:"runs_count"`
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
which thread and conversation, and how it ended.
*/
type OperatorUsageRun struct {
	ID                 string    `json:"id"`
	Purpose            string    `json:"purpose"`
	Status             string    `json:"status"`
	MemberName         string    `json:"member_name"`
	ThreadTitle        string    `json:"thread_title"`
	ConversationID     *string   `json:"conversation_id"`
	ConversationNumber *int64    `json:"conversation_number"`
	LatencyMs          *int32    `json:"latency_ms"`
	CreatedAt          time.Time `json:"created_at"`
	ThreadID           *string   `json:"thread_id"`
	RoutineID          *string   `json:"routine_id"`
}

/*
NewOperatorUsage shapes the usage view.
*/
func NewOperatorUsage(v operatorchat.UsageView) OperatorUsage {
	t := v.Totals
	out := OperatorUsage{
		PeriodStart: v.Start.Format("2006-01-02"), RunsCount: t.RunsCount,
		FailedCount: t.FailedCount, InputTokens: t.InputTokens, OutputTokens: t.OutputTokens,

		Daily: make([]AiUsageDay, len(v.Daily)), ByPurpose: make([]OperatorUsagePurpose, len(v.ByPurpose)),
		ByMember: make([]OperatorUsageMember, len(v.ByMember)), ByModel: make([]AiUsageModel, len(v.ByModel)),
		Changesets: make([]OperatorUsageChangesets, len(v.Changesets)),
		Recent:     make([]OperatorUsageRun, len(v.Recent)),
		ByRoutine:  make([]OperatorUsageRoutine, len(v.ByRoutine)),
	}
	for i, r := range v.ByRoutine {
		out.ByRoutine[i] = OperatorUsageRoutine{
			RoutineID: models.PublicID(models.PrefixOperatorRoutine, r.ID), Name: r.Name, RunsCount: r.RunsCount,
		}
	}
	for i, d := range v.Daily {
		out.Daily[i] = AiUsageDay{
			Day: d.Day.Time.Format("2006-01-02"), RunsCount: int(d.RunsCount),
		}
	}
	for i, p := range v.ByPurpose {
		out.ByPurpose[i] = OperatorUsagePurpose{Purpose: p.Purpose, RunsCount: p.RunsCount}
	}
	for i, m := range v.ByMember {
		out.ByMember[i] = OperatorUsageMember{
			MemberID: models.PublicID(models.PrefixMember, m.MemberID), Name: m.Name,
			RunsCount: m.RunsCount,
		}
	}
	for i, m := range v.ByModel {
		out.ByModel[i] = AiUsageModel{Model: m.Model, Calls: int(m.Calls)}
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
		LatencyMs: r.LatencyMs, CreatedAt: r.CreatedAt,
		ThreadID:  optionalID(models.PrefixOperatorThread, r.ThreadID),
		RoutineID: optionalID(models.PrefixOperatorRoutine, r.RoutineID),
	}
	if r.ConversationID != nil {
		id := models.PublicID(models.PrefixConversation, *r.ConversationID)
		run.ConversationID = &id
	}
	return run
}
