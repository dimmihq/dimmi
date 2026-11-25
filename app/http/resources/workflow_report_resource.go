/*
|--------------------------------------------------------------------------
| Workflow Report Resources
|--------------------------------------------------------------------------
|
| One workflow in the workflows report.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/workflows"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WorkflowReportRow is one workflow in the workflows report: how many
runs it started in the range, how many completed, went to a person or
failed, and how many reached its goal.
*/
type WorkflowReportRow struct {
	WorkflowID string        `json:"workflow_id"`
	Name       string        `json:"name"`
	Trigger    string        `json:"trigger"`
	Status     string        `json:"status"`
	Triggered  int32         `json:"triggered"`
	Completed  int32         `json:"completed"`
	HandedOver int32         `json:"handed_over"`
	Failed     int32         `json:"failed"`
	Goal       *WorkflowGoal `json:"goal"`
	GoalMet    *int32        `json:"goal_met"`
}

/*
NewWorkflowReportRow shapes one workflow of the report.
*/
func NewWorkflowReportRow(r workflows.ReportRow) WorkflowReportRow {
	m := NewWorkflowMetrics(r.Workflow.ID, workflows.Metrics{Goal: r.Goal, Figures: r.Figures})

	return WorkflowReportRow{
		WorkflowID: models.PublicID(models.PrefixWorkflow, r.Workflow.ID), Name: r.Workflow.Name,
		Trigger: r.Workflow.Trigger, Status: r.Workflow.Status, Triggered: r.Figures.Triggered,
		Completed: r.Figures.Completed, HandedOver: r.Figures.HandedOver, Failed: r.Figures.Failed,
		Goal: m.Goal, GoalMet: m.GoalMet,
	}
}
