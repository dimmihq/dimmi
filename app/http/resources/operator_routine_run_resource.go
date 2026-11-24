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

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OperatorRoutineRun is one run of a routine: what started it, how it
went, what it suggested and cost, the thread it wrote on, and the steps
it has finished, which grow while it runs.
*/
type OperatorRoutineRun struct {
	ID              string     `json:"id"`
	Object          string     `json:"object"`
	RoutineID       string     `json:"routine_id"`
	Trigger         string     `json:"trigger"`
	TriggeredBy     *string    `json:"triggered_by"`
	ScheduledFor    *time.Time `json:"scheduled_for"`
	ThreadID        *string    `json:"thread_id"`
	AIRunID         *string    `json:"ai_run_id"`
	Status          string     `json:"status"`
	Outcome         *string    `json:"outcome"`
	NoopReason      *string    `json:"noop_reason"`
	SkipReason      *string    `json:"skip_reason"`
	ErrorCode       *string    `json:"error_code"`
	ErrorMessage    *string    `json:"error_message"`
	ChangesetsCount int32      `json:"changesets_count"`
	// How many of the run's suggestions nobody has applied or declined yet.
	SuggestionsWaiting int32 `json:"suggestions_waiting"`
	// What the run read to do its work, counted once each.
	Read            OperatorRoutineRunRead `json:"read"`
	CostMicros      int64                  `json:"cost_micros"`
	Notified        bool                   `json:"notified"`
	DeliveryResults json.RawMessage        `json:"delivery_results"`
	Steps           json.RawMessage        `json:"steps"`
	StartedAt       *time.Time             `json:"started_at"`
	CompletedAt     *time.Time             `json:"completed_at"`
	CreatedAt       time.Time              `json:"created_at"`
}

/*
OperatorRoutineRunRead is how much a run read: the articles,
conversations and procedures it opened, each counted once.
*/
type OperatorRoutineRunRead struct {
	Articles      int `json:"articles"`
	Conversations int `json:"conversations"`
	Procedures    int `json:"procedures"`
}

/*
readKind is what a tool opens one of: an article, a conversation or a
procedure, under the name the tool has and the name it had; empty for
any other tool.
*/
func readKind(tool string) string {
	switch tool {
	case "get_article_by_id", "get_article":
		return "article"
	case "get_conversation_by_number", "get_conversation":
		return "conversation"
	case "get_procedure_by_id", "get_procedure":
		return "procedure"
	}
	return ""
}

/*
readOf counts what a run's steps read. A step's summary names what it
opened, so the same article read twice, by the run or by two of its
subagents, counts once.
*/
func readOf(steps json.RawMessage) OperatorRoutineRunRead {
	var list []struct {
		Tool    string `json:"tool"`
		Summary string `json:"summary"`
		OK      bool   `json:"ok"`
	}
	_ = json.Unmarshal(steps, &list)
	seen := map[string]bool{}
	var out OperatorRoutineRunRead
	for _, s := range list {
		kind := readKind(s.Tool)
		if kind == "" || !s.OK || seen[kind+s.Summary] {
			continue
		}
		seen[kind+s.Summary] = true
		switch kind {
		case "article":
			out.Articles++
		case "conversation":
			out.Conversations++
		default:
			out.Procedures++
		}
	}
	return out
}

/*
NewOperatorRoutineRun shapes a run, with how many of its suggestions
still wait for a teammate.
*/
func NewOperatorRoutineRun(r models.OperatorRoutineRun, waiting int32) OperatorRoutineRun {
	out := OperatorRoutineRun{
		ID: models.PublicID(models.PrefixOperatorRoutineRun, r.ID), Object: "operator_routine_run",
		RoutineID: models.PublicID(models.PrefixOperatorRoutine, r.RoutineID), Trigger: r.Trigger,
		TriggeredBy:  optionalID(models.PrefixMember, r.TriggeredByMemberID),
		ScheduledFor: r.ScheduledFor, ThreadID: optionalID(models.PrefixOperatorThread, r.ThreadID),
		AIRunID: optionalID(models.PrefixAiRun, r.AiRunID), Status: r.Status, Outcome: r.Outcome,
		NoopReason: r.NoopReason, SkipReason: r.SkipReason, ErrorCode: r.ErrorCode, ErrorMessage: r.ErrorMessage,
		ChangesetsCount: r.ChangesetsCount, CostMicros: r.CostMicros, Notified: r.Notified,
		DeliveryResults: r.DeliveryResults, Steps: r.Steps, StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
		CreatedAt: r.CreatedAt, SuggestionsWaiting: waiting, Read: readOf(r.Steps),
	}
	if len(out.DeliveryResults) == 0 {
		out.DeliveryResults = json.RawMessage("[]")
	}
	if len(out.Steps) == 0 {
		out.Steps = json.RawMessage("[]")
	}
	return out
}

/*
NewOperatorRoutineRuns shapes runs, with the suggestions of each still
waiting, by run.
*/
func NewOperatorRoutineRuns(rows []models.OperatorRoutineRun, waiting map[ids.UUID]int32) []OperatorRoutineRun {
	out := make([]OperatorRoutineRun, len(rows))
	for i, r := range rows {
		out[i] = NewOperatorRoutineRun(r, waiting[r.ID])
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
			Prompt: t.Prompt, Cadence: t.Cadence, CronExpression: t.Expr, NotifyPolicy: t.NotifyPolicy,
			Requires: orStrings(t.Requires),
		}
	}
	return out
}
