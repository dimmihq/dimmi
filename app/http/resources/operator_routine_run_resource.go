/*
|--------------------------------------------------------------------------
| Operator Routine Run Resources
|--------------------------------------------------------------------------
|
| A routine's run reads with its outcome, cost and thread; a template
| with the prompt, cadence and access it fills in.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorRoutineRun is one run of a routine: what started it, how it
went, what it suggested and cost, and the thread it wrote on.
*/
type OperatorRoutineRun struct {
	ID              string          `json:"id"`
	Object          string          `json:"object"`
	RoutineID       string          `json:"routine_id"`
	Trigger         string          `json:"trigger"`
	TriggeredBy     *string         `json:"triggered_by"`
	ScheduledFor    *time.Time      `json:"scheduled_for"`
	ThreadID        *string         `json:"thread_id"`
	AIRunID         *string         `json:"ai_run_id"`
	Status          string          `json:"status"`
	Outcome         *string         `json:"outcome"`
	NoopReason      *string         `json:"noop_reason"`
	SkipReason      *string         `json:"skip_reason"`
	ErrorCode       *string         `json:"error_code"`
	ErrorMessage    *string         `json:"error_message"`
	ChangesetsCount int32           `json:"changesets_count"`
	CostMicros      int64           `json:"cost_micros"`
	Notified        bool            `json:"notified"`
	DeliveryResults json.RawMessage `json:"delivery_results"`
	StartedAt       *time.Time      `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at"`
	CreatedAt       time.Time       `json:"created_at"`
}

/*
NewOperatorRoutineRun shapes a run.
*/
func NewOperatorRoutineRun(r models.OperatorRoutineRun) OperatorRoutineRun {
	out := OperatorRoutineRun{
		ID: models.PublicID(models.PrefixOperatorRoutineRun, r.ID), Object: "operator_routine_run",
		RoutineID: models.PublicID(models.PrefixOperatorRoutine, r.RoutineID), Trigger: r.Trigger,
		TriggeredBy:  optionalID(models.PrefixMember, r.TriggeredByMemberID),
		ScheduledFor: r.ScheduledFor, ThreadID: optionalID(models.PrefixOperatorThread, r.ThreadID),
		AIRunID: optionalID(models.PrefixAiRun, r.AiRunID), Status: r.Status, Outcome: r.Outcome,
		NoopReason: r.NoopReason, SkipReason: r.SkipReason, ErrorCode: r.ErrorCode, ErrorMessage: r.ErrorMessage,
		ChangesetsCount: r.ChangesetsCount, CostMicros: r.CostMicros, Notified: r.Notified,
		DeliveryResults: r.DeliveryResults, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
		CreatedAt: r.CreatedAt,
	}
	if len(out.DeliveryResults) == 0 {
		out.DeliveryResults = json.RawMessage("[]")
	}
	return out
}

/*
NewOperatorRoutineRuns shapes runs.
*/
func NewOperatorRoutineRuns(rows []models.OperatorRoutineRun) []OperatorRoutineRun {
	out := make([]OperatorRoutineRun, len(rows))
	for i, r := range rows {
		out[i] = NewOperatorRoutineRun(r)
	}
	return out
}

/*
OperatorRoutineTemplate is a routine filled in ahead. Requires names
what the workspace must have connected for it to work: github,
help_center, conversations or procedures.
*/
type OperatorRoutineTemplate struct {
	Key            string   `json:"key"`
	Object         string   `json:"object"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Prompt         string   `json:"prompt"`
	Cadence        string   `json:"cadence"`
	CronExpression string   `json:"cron_expression"`
	ToolGroups     []string `json:"tool_groups"`
	Permissions    []string `json:"permissions"`
	NotifyPolicy   string   `json:"notify_policy"`
	Requires       []string `json:"requires"`
}

/*
NewOperatorRoutineTemplates shapes the templates.
*/
func NewOperatorRoutineTemplates(list []operator.RoutineTemplate) []OperatorRoutineTemplate {
	out := make([]OperatorRoutineTemplate, len(list))
	for i, t := range list {
		out[i] = OperatorRoutineTemplate{
			Key: t.Key, Object: "operator_routine_template", Name: t.Name, Description: t.Description,
			Prompt: t.Prompt, Cadence: t.Cadence, CronExpression: t.Expr, ToolGroups: orStrings(t.ToolGroups),
			Permissions: orStrings(t.Permissions), NotifyPolicy: t.NotifyPolicy, Requires: orStrings(t.Requires),
		}
	}
	return out
}
