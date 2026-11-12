/*
|--------------------------------------------------------------------------
| Slow Steps Off The Conversation
|--------------------------------------------------------------------------
|
| A request to the customer's system can take ten seconds and sorting by
| topic three, too long to hold the conversation and a database
| connection. Such a step only waits; a job reads what the call needs in
| a short transaction of its own, makes the call with no transaction
| open, and keeps the answer on the run's wait before queueing the run
| to go on with it. A retried job finds the answer already kept and
| never calls twice for the same wait; a run that moved on or ended
| meanwhile simply ignores the answer. Erasing the customer forgets an
| answer kept for them, and one that comes back after they were erased
| is never kept: the step then goes on as if the call failed.
|
*/

package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
waitCallOut is the wait of a run for a slow step's call.
*/
const waitCallOut = "call_out"

/*
InputCallOutDone tells a run the call it waited on was answered.
*/
const InputCallOutDone = "call_out_done"

/*
callAnswer is what a slow step's call came back with. A request keeps
its status, the values it saves or why it failed; sorting keeps the
category and confidence, or why it went unsorted. Only the mapped
values of a response are kept, never its body.
*/
type callAnswer struct {
	Error      string       `json:"error,omitempty"`
	Status     int          `json:"status,omitempty"`
	Saves      []savedValue `json:"saves,omitempty"`
	Category   string       `json:"category,omitempty"`
	Confidence string       `json:"confidence,omitempty"`
	Reason     string       `json:"reason,omitempty"`
}

/*
savedValue is one response value and the field it goes to.
*/
type savedValue struct {
	Field string `json:"field"`
	Value any    `json:"value"`
}

/*
callingOut is the outcome of a slow step: the run waits for its call.
*/
func callingOut(acted bool) outcome {
	return outcome{outcome: "calling", acted: acted, wait: &waitState{Kind: waitCallOut}}
}

/*
call is what a slow step's call needs, read before it is made: the
request to send or the question to ask, or the answer when there is
nothing to call.
*/
type call struct {
	kind     string
	request  outbound
	saves    []workflowspec.ResponseSave
	timeout  string
	question ClassifyQuestion
	answer   *callAnswer
	contact  *ids.UUID
}

/*
CallOut makes the call a run waits on and keeps its answer for the
run.
*/
func (e *Engine) CallOut(ctx context.Context, a jobs.CallOutForWorkflowArgs) error {
	c, err := e.prepareCall(ctx, a)
	if err != nil || c == nil {
		return err
	}

	answer := c.answer
	if answer == nil {
		answer = e.makeCall(ctx, c)
	}

	return e.keepAnswer(ctx, a, c.contact, *answer)
}

/*
prepareCall reads what the call needs in a short transaction that
takes no lock, nil when the run no longer waits on it or already has
its answer.
*/
func (e *Engine) prepareCall(ctx context.Context, a jobs.CallOutForWorkflowArgs) (*call, error) {
	var c *call
	err := e.transaction(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		run, err := models.New(tx).FindWorkflowRun(ctx, models.FindWorkflowRunParams{
			WorkspaceID: a.WorkspaceID, ID: a.RunID,
		})
		if errors.Is(err, database.ErrNotFound) || err == nil && !awaitsCall(run, a) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find workflow run: %w", err)
		}

		st, err := e.load(ctx, tx, run, nil)
		if err != nil {
			return err
		}

		step, ok := st.version.index.Step(a.StepID)
		if !ok {
			c = &call{answer: &callAnswer{Error: "unavailable", Reason: "ai_unavailable"}}
		} else if c, err = st.prepare(ctx, step); err != nil || c == nil {
			return err
		}

		c.contact = run.ContactID

		return nil
	})

	return c, err
}

/*
awaitsCall reports whether the run still waits on the job's call with
no answer yet.
*/
func awaitsCall(run models.WorkflowRun, a jobs.CallOutForWorkflowArgs) bool {
	w := readWait(run.Wait)

	return run.Seq == a.Seq && run.Status == StatusWaiting && w.Kind == waitCallOut && w.StepID == a.StepID &&
		w.Answer == nil
}

/*
prepare reads what the step's call needs.
*/
func (st *state) prepare(ctx context.Context, s workflowspec.Step) (*call, error) {
	if s.Kind == workflowspec.KindClassify {
		return st.prepareQuestion(ctx, s)
	}

	return st.prepareRequest(ctx, s)
}

/*
makeCall makes the call, with no transaction open.
*/
func (e *Engine) makeCall(ctx context.Context, c *call) *callAnswer {
	if c.kind == workflowspec.KindClassify {
		return e.ask(ctx, c.question)
	}

	return e.sendRequest(ctx, c)
}

/*
keepAnswer keeps the answer on the run's wait and queues the run to go
on with it, unless the run moved on or a retry already kept one. An
answer about a customer erased since the call was prepared is not
kept.
*/
func (e *Engine) keepAnswer(
	ctx context.Context, a jobs.CallOutForWorkflowArgs, contact *ids.UUID, answer callAnswer,
) error {
	return e.transaction(ctx, a.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		run, err := q.LockWorkflowRun(ctx, models.LockWorkflowRunParams{WorkspaceID: a.WorkspaceID, ID: a.RunID})
		if errors.Is(err, database.ErrNotFound) || err == nil && !awaitsCall(run, a) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("lock workflow run: %w", err)
		}

		if !sameContact(run.ContactID, contact) {
			answer = erasedAnswer()
		}

		w := readWait(run.Wait)
		w.Answer = &answer

		wait, err := json.Marshal(w)
		if err != nil {
			return fmt.Errorf("encode wait: %w", err)
		}

		if err := q.KeepWorkflowRunWait(ctx, models.KeepWorkflowRunWaitParams{
			Wait: wait, Now: e.Clock.Now(), WorkspaceID: run.WorkspaceID, ID: run.ID, Seq: run.Seq,
		}); err != nil {
			return fmt.Errorf("keep call answer: %w", err)
		}

		return e.Queue.PushTx(ctx, tx, jobs.AdvanceWorkflowRunArgs{
			WorkspaceID: run.WorkspaceID, RunID: run.ID, Seq: run.Seq, Input: InputCallOutDone,
		})
	})
}

/*
sameContact reports whether the run is still for the contact the call
was prepared for.
*/
func sameContact(now, before *ids.UUID) bool {
	if now == nil || before == nil {
		return now == nil && before == nil
	}

	return *now == *before
}

/*
erasedAnswer is the answer of a call whose customer was erased: a
request fails and sorting goes on unsorted.
*/
func erasedAnswer() callAnswer {
	return callAnswer{Error: "answer_erased", Reason: "answer_erased"}
}

/*
answered returns the answer the run's wait kept; erasing the customer
removes it, and the step then goes on as if the call failed.
*/
func (st *state) answered() callAnswer {
	if st.wait.Answer == nil {
		return erasedAnswer()
	}

	return *st.wait.Answer
}
