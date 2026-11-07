/*
|--------------------------------------------------------------------------
| Action Call Jobs
|--------------------------------------------------------------------------
|
| The three jobs that carry an agent action past the conversation turn
| that requested it: running an approved call once, waking the agent to
| tell the customer how it ended, and expiring a call nobody decided in
| time. Each runs inside its workspace, so row security applies.
|
*/

package aijobs

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/actionkit"
	"github.com/dimmihq/dimmi/app/ai/pipeline"
)

/*
ExecuteAiActionCall runs one approved call.
*/
type ExecuteAiActionCall struct {
	Kit *actionkit.Kit
}

/*
Handle runs the call.
*/
func (w *ExecuteAiActionCall) Handle(ctx context.Context, job *queue.Job[actionkit.ExecuteCallArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())
	return w.Kit.Execute(ctx, job.Args.WorkspaceID, job.Args.CallID)
}

/*
FollowUpAiActionCall wakes the agent once a call has ended.
*/
type FollowUpAiActionCall struct {
	Runner *pipeline.Runner
}

/*
Handle runs the follow-up.
*/
func (w *FollowUpAiActionCall) Handle(ctx context.Context, job *queue.Job[actionkit.FollowUpArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())
	_, err := w.Runner.RunActionFollowUp(ctx, job.Args.WorkspaceID, job.Args.CallID)
	return err
}

/*
ExpireAiActionCall ends a call at its deadline, or waits for the
deadline when the job ran early.
*/
type ExpireAiActionCall struct {
	Kit *actionkit.Kit
}

/*
Handle expires the call.
*/
func (w *ExpireAiActionCall) Handle(ctx context.Context, job *queue.Job[actionkit.ExpireCallArgs]) error {
	ctx = tenancy.With(ctx, job.Args.WorkspaceID.String())
	wait, err := w.Kit.Expire(ctx, job.Args.WorkspaceID, job.Args.CallID, time.UnixMilli(job.Args.At))
	if err != nil {
		return err
	}
	if wait > 0 {
		return queue.Postpone(wait)
	}
	return nil
}
