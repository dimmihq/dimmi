/*
|--------------------------------------------------------------------------
| Resuming and Interrupting Runs
|--------------------------------------------------------------------------
|
| A customer message resumes the runs of its conversation that wait for
| an answer, or whose wait ends early when the customer writes; the
| agent handing a conversation over resumes the runs waiting on it. People
| win over scripts: a teammate's public reply ends every
| customer-facing run on the conversation at once, in the transaction
| of the reply, so the customer never gets a scripted message after a
| person answered; background runs only hear about it if they wait for
| it. Closing, spam and deletion end every run.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
MatchEvent matches an event a job carries.
*/
func (e *Engine) MatchEvent(ctx context.Context, raw json.RawMessage) error {
	var ev Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return fmt.Errorf("read workflow event: %w", err)
	}

	return e.Match(ctx, ev)
}

/*
AdvanceRuns advances the runs an event started, in order.
*/
func (e *Engine) AdvanceRuns(ctx context.Context, workspace ids.UUID, runs []ids.UUID) error {
	return e.AdvanceBatch(ctx, workspace, runs)
}

/*
AdvanceRun moves a run on at its sequence, or wakes it. A timer that
comes before its run's wake time waits until it is due rather than
being spent early.
*/
func (e *Engine) AdvanceRun(ctx context.Context, a jobs.AdvanceWorkflowRunArgs) error {
	if a.Wake {
		if early, err := e.earlyBy(ctx, a); err != nil || early > 0 {
			return errors.Join(err, postponed(early))
		}
	}

	s := Step{WorkspaceID: a.WorkspaceID, RunID: a.RunID, Seq: &a.Seq}
	switch {
	case a.Wake:
		s.Input = &Input{Kind: InputTimer}
	case a.Input != "":
		s.Input = &Input{Kind: a.Input}
	}

	return e.Advance(ctx, s)
}

/*
earlyBy returns how long before its run's wake time a timer came, zero
when it is due or no longer applies.
*/
func (e *Engine) earlyBy(ctx context.Context, a jobs.AdvanceWorkflowRunArgs) (time.Duration, error) {
	var early time.Duration
	err := e.transaction(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		run, err := models.New(tx).FindWorkflowRun(ctx, models.FindWorkflowRunParams{
			WorkspaceID: a.WorkspaceID, ID: a.RunID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find workflow run: %w", err)
		}

		if run.Seq == a.Seq && run.Status == StatusWaiting && run.WakeAt != nil {
			early = run.WakeAt.Sub(e.Clock.Now())
		}

		return nil
	})

	return max(early, 0), err
}

/*
postponed holds the job back for the duration, nothing for none.
*/
func postponed(d time.Duration) error {
	if d <= 0 {
		return nil
	}

	return queue.Postpone(d)
}

/*
ResumeOnMessage resumes the conversation's runs that wait for the
customer.
*/
func (e *Engine) ResumeOnMessage(ctx context.Context, workspace, conversation, message ids.UUID) error {
	return e.resumeWaiting(ctx, workspace, conversation, Input{Kind: InputMessage, MessageID: &message})
}

/*
ResumeOnAgent tells the conversation's runs waiting on the agent that
it handed the conversation over, and why.
*/
func (e *Engine) ResumeOnAgent(ctx context.Context, workspace, conversation ids.UUID, reason string) error {
	return e.resumeWaiting(ctx, workspace, conversation, Input{Kind: InputAgentHandover, Reason: reason})
}

/*
resumeWaiting hands the input to every waiting run of the conversation;
each run takes only what it waits for.
*/
func (e *Engine) resumeWaiting(ctx context.Context, workspace, conversation ids.UUID, in Input) error {
	var waiting []ids.UUID
	err := e.transaction(ctx, workspace, func(ctx context.Context, tx *database.Tx) error {
		runs, err := models.New(tx).ActiveWorkflowRuns(ctx, models.ActiveWorkflowRunsParams{
			WorkspaceID: workspace, ConversationID: &conversation,
		})
		for _, r := range runs {
			if r.Status == StatusWaiting {
				waiting = append(waiting, r.ID)
			}
		}

		return err
	})
	if err != nil {
		return fmt.Errorf("find waiting runs: %w", err)
	}

	for _, run := range waiting {
		input := in
		if err := e.Advance(ctx, Step{WorkspaceID: workspace, RunID: run, Input: &input}); err != nil {
			return err
		}
	}

	return nil
}

/*
Interruption is a change that reaches a conversation's runs: the
conversation, the reason runs end with, and the chain of workflows
that made the change.
*/
type Interruption struct {
	Conversation models.Conversation
	Reason       string
	Chain        []ids.UUID
}

/*
InterruptInTx ends or interrupts the conversation's runs in the
caller's transaction. A teammate's reply ends customer-facing runs and
interrupts background waits that listen for it; anything else that
takes the conversation away ends every run with the reason.
*/
func (e *Engine) InterruptInTx(ctx context.Context, tx *database.Tx, in Interruption) error {
	conv := in.Conversation
	runs, err := models.New(tx).ActiveWorkflowRuns(ctx, models.ActiveWorkflowRunsParams{
		WorkspaceID: conv.WorkspaceID, ConversationID: &conv.ID,
	})
	if err != nil {
		return fmt.Errorf("find active runs: %w", err)
	}

	for _, run := range runs {
		if err := e.interrupt(ctx, tx, run, in); err != nil {
			return err
		}
	}

	return nil
}

/*
interrupt applies one change to one run.
*/
func (e *Engine) interrupt(ctx context.Context, tx *database.Tx, run models.WorkflowRun, in Interruption) error {
	reason := in.Reason
	if causedBy(run, in.Chain) {
		return nil
	}

	if reason == EndTeammateReplied && run.Kind != workflowspec.KindCustomerFacing {
		if run.Status != StatusWaiting || !readWait(run.Wait).interrupts(interruptTeammate) {
			return nil
		}

		return e.Queue.PushTx(ctx, tx, jobs.AdvanceWorkflowRunArgs{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Seq: run.Seq, Input: InputTeammateReply,
		})
	}

	return e.end(ctx, tx, run, reason)
}

/*
causedBy reports whether the change came from the run's own workflow,
which never interrupts itself.
*/
func causedBy(run models.WorkflowRun, chain []ids.UUID) bool {
	return len(chain) > 0 && chain[len(chain)-1] == run.WorkflowID
}

/*
end ends a run from outside, letting go of its conversation.
*/
func (e *Engine) end(ctx context.Context, tx *database.Tx, run models.WorkflowRun, reason string) error {
	q, now := models.New(tx), e.Clock.Now()
	if _, err := q.SaveWorkflowRun(ctx, models.SaveWorkflowRunParams{
		Status: statusFor(reason), EndReason: &reason, Vars: run.Vars, StepsExecuted: run.StepsExecuted,
		Acted: run.Acted, EndedAt: &now, Now: now, WorkspaceID: run.WorkspaceID, ID: run.ID,
	}); err != nil {
		return fmt.Errorf("end workflow run: %w", err)
	}

	if err := q.FinishWorkflowRunStep(ctx, models.FinishWorkflowRunStepParams{
		Status: "interrupted", Outcome: &reason, Detail: json.RawMessage("{}"), Now: &now,
		WorkspaceID: run.WorkspaceID, RunID: run.ID, StepID: deref(run.CursorStepID),
	}); err != nil {
		return fmt.Errorf("close step trace: %w", err)
	}

	if run.Kind != workflowspec.KindCustomerFacing {
		return nil
	}

	return q.ReleaseConversationClaim(ctx, models.ReleaseConversationClaimParams{
		Now: now, WorkspaceID: run.WorkspaceID, ConversationID: *run.ConversationID, RunID: &run.ID,
	})
}
