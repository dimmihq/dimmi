/*
|--------------------------------------------------------------------------
| Weekly Procedure Review
|--------------------------------------------------------------------------
|
| Once a week every workspace gets a job of its own in which Operator
| reviews how its live procedures did, when the workspace turned that
| on; a workspace that did not skips it cheaply. Operator only proposes:
| the team reviews every fix and test before anything changes, and the
| review spends from the month's AI budget like any other run.
|
*/

package aijobs

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReviewProceduresWeeklyArgs names the weekly round; it takes no
arguments.
*/
type ReviewProceduresWeeklyArgs struct{}

/*
Kind names the job.
*/
func (ReviewProceduresWeeklyArgs) Kind() string { return "review_procedures_weekly" }

/*
ReviewProceduresWeekly queues one review per workspace.
*/
type ReviewProceduresWeekly struct {
	DB    *database.DB
	Queue *queue.Queue
}

/*
Handle queues the workspaces' reviews.
*/
func (w *ReviewProceduresWeekly) Handle(ctx context.Context, _ *queue.Job[ReviewProceduresWeeklyArgs]) error {
	workspaces, err := models.New(w.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces to review: %w", err)
	}
	for _, ws := range workspaces {
		if err := w.Queue.Push(ctx, ReviewProceduresArgs{WorkspaceID: ws.ID}); err != nil {
			return fmt.Errorf("queue procedure review: %w", err)
		}
	}
	return nil
}

/*
ReviewProceduresArgs names the workspace whose procedures to review.
*/
type ReviewProceduresArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
}

/*
Kind names the job.
*/
func (ReviewProceduresArgs) Kind() string { return "review_procedures" }

/*
ReviewProcedures runs Operator's review of one workspace's procedures.
*/
type ReviewProcedures struct {
	Operator *operator.Engine
}

/*
Handle runs the review.
*/
func (w *ReviewProcedures) Handle(ctx context.Context, job *queue.Job[ReviewProceduresArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())
	return w.Operator.ReviewProcedures(ctx, job.Args.WorkspaceID)
}
