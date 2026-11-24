/*
|--------------------------------------------------------------------------
| Workflow Analytics Resources
|--------------------------------------------------------------------------
|
| A workflow's numbers as the analytics view reads them: totals over
| the range, the goal they were counted against, runs per day, and per
| step how many reached it, wait on it, failed on it, ended on it and
| left by each path. A failure is one run that failed, with its first
| failing step and the conversation to open.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkflowMetrics is a workflow's numbers over a range.
*/
type WorkflowMetrics struct {
	Object     string                         `json:"object"`
	WorkflowID string                         `json:"workflow_id"`
	From       time.Time                      `json:"from"`
	To         time.Time                      `json:"to"`
	Version    *int32                         `json:"version"`
	Goal       *WorkflowGoal                  `json:"goal"`
	Triggered  int32                          `json:"triggered"`
	Engaged    int32                          `json:"engaged"`
	Completed  int32                          `json:"completed"`
	Active     int32                          `json:"active"`
	HandedOver int32                          `json:"handed_over"`
	Failed     int32                          `json:"failed"`
	GoalMet    *int32                         `json:"goal_met"`
	Days       []WorkflowMetricsDay           `json:"days"`
	Steps      map[string]WorkflowStepMetrics `json:"steps"`
}

/*
WorkflowGoal is the goal runs were counted against.
*/
type WorkflowGoal struct {
	Kind      string `json:"kind"`
	Window    string `json:"window"`
	MinRating *int   `json:"min_rating"`
}

/*
WorkflowMetricsDay is one day's runs.
*/
type WorkflowMetricsDay struct {
	Day       time.Time `json:"day"`
	Triggered int32     `json:"triggered"`
	Completed int32     `json:"completed"`
	Failed    int32     `json:"failed"`
}

/*
WorkflowStepMetrics is one step's numbers.
*/
type WorkflowStepMetrics struct {
	Reached int32            `json:"reached"`
	Waiting int32            `json:"waiting"`
	Failed  int32            `json:"failed"`
	Dropped int32            `json:"dropped"`
	Paths   map[string]int32 `json:"paths"`
}

/*
NewWorkflowMetrics shapes a workflow's numbers.
*/
func NewWorkflowMetrics(id ids.UUID, m workflows.Metrics) WorkflowMetrics {
	f := m.Figures
	out := WorkflowMetrics{
		Object: "workflow_metrics", WorkflowID: models.PublicID(models.PrefixWorkflow, id), From: m.From, To: m.To,
		Triggered: f.Triggered, Engaged: f.Engaged, Completed: f.Completed, Active: f.Active,
		HandedOver: f.HandedOver, Failed: f.Failed, Days: []WorkflowMetricsDay{},
		Steps: map[string]WorkflowStepMetrics{},
	}

	if m.Version > 0 {
		out.Version = &m.Version
	}

	if g := m.Goal; g != nil {
		goal := WorkflowGoal{Kind: g.Kind, Window: g.Window}
		if goal.Window == "" {
			goal.Window = "7d"
		}

		if g.MinRating > 0 {
			goal.MinRating = &g.MinRating
		}

		out.Goal, out.GoalMet = &goal, &f.Goal
	}

	for _, d := range m.Days {
		out.Days = append(out.Days, WorkflowMetricsDay{
			Day: d.Day, Triggered: d.Triggered, Completed: d.Completed, Failed: d.Failed,
		})
	}

	for stepID, st := range m.Steps {
		out.Steps[stepID] = WorkflowStepMetrics{
			Reached: st.Reached, Waiting: st.Waiting, Failed: st.Failed, Dropped: st.Dropped, Paths: st.Paths,
		}
	}

	return out
}

/*
WorkflowFailure is one run that failed: how it ended, its first failing
step and what that step kept, and the conversation it worked.
*/
type WorkflowFailure struct {
	ID                 string          `json:"id"`
	Object             string          `json:"object"`
	RunID              string          `json:"run_id"`
	Status             string          `json:"status"`
	EndReason          *string         `json:"end_reason"`
	Error              json.RawMessage `json:"error"`
	ConversationID     *string         `json:"conversation_id"`
	ConversationNumber *int64          `json:"conversation_number"`
	VersionNumber      *int32          `json:"version_number"`
	StepID             *string         `json:"step_id"`
	StepKind           *string         `json:"step_kind"`
	Outcome            *string         `json:"outcome"`
	Detail             json.RawMessage `json:"detail"`
	StartedAt          time.Time       `json:"started_at"`
	EndedAt            *time.Time      `json:"ended_at"`
}

/*
NewWorkflowFailures shapes failed runs.
*/
func NewWorkflowFailures(rows []models.WorkflowFailuresRow) []WorkflowFailure {
	out := make([]WorkflowFailure, 0, len(rows))
	for _, r := range rows {
		run := models.PublicID(models.PrefixWorkflowRun, r.ID)
		f := WorkflowFailure{
			ID: run, Object: "workflow_failure", RunID: run, Status: r.Status, EndReason: r.EndReason,
			Error: r.Error, ConversationID: publicOrNil(models.PrefixConversation, r.ConversationID),
			Detail: r.StepDetail, StartedAt: r.StartedAt, EndedAt: r.EndedAt,
		}

		if len(f.Error) == 0 {
			f.Error = json.RawMessage("null")
		}

		if r.ConversationNumber > 0 {
			f.ConversationNumber = &r.ConversationNumber
		}

		if r.VersionNumber > 0 {
			f.VersionNumber = &r.VersionNumber
		}

		if r.StepID != "" {
			f.StepID, f.StepKind, f.Outcome = &r.StepID, &r.StepKind, nonEmpty(r.StepOutcome)
		}

		out = append(out, f)
	}

	return out
}
