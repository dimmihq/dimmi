/*
|--------------------------------------------------------------------------
| Split Variant Resources
|--------------------------------------------------------------------------
|
| How each variant of an A/B split did: the runs that took it and those
| whose conversation reached the workflow's goal, so the variants can be
| compared side by side.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/models"

/*
WorkflowVariantMetrics is one variant of a split: the runs that took
it and those that reached the goal.
*/
type WorkflowVariantMetrics struct {
	Reached int32 `json:"reached"`
	Goal    int32 `json:"goal"`
}

/*
newWorkflowVariants shapes a split's variants by path id, nil for a
step that is not a split.
*/
func newWorkflowVariants(rows map[string]models.WorkflowVariantGoalsRow) map[string]WorkflowVariantMetrics {
	if len(rows) == 0 {
		return nil
	}

	out := make(map[string]WorkflowVariantMetrics, len(rows))
	for pathID, v := range rows {
		out[pathID] = WorkflowVariantMetrics{Reached: v.Reached, Goal: v.Goal}
	}

	return out
}
