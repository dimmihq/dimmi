/*
|--------------------------------------------------------------------------
| Advancing Runs
|--------------------------------------------------------------------------
|
| Advancing a run takes the conversation's lock, so one run moves at a
| time per conversation, then executes steps until one waits, the path
| ends or the job has done its share. Every step leaves a trace row and
| moves the run's sequence on in the same transaction as its effect: a
| job that carries an old sequence finds nothing to do, so retries
| never repeat an effect.
|
| A run waiting for an answer or for time is resumed with what it waited
| for; the step it waits on decides where the run goes next.
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

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
The run statuses.
*/
const (
	StatusRunning   = "running"
	StatusWaiting   = "waiting"
	StatusCompleted = "completed"
	StatusPassed    = "passed"
	StatusExited    = "exited"
	StatusCanceled  = "canceled"
	StatusFailed    = "failed"
	StatusExpired   = "expired"
)

/*
The end reasons a run records.
*/
const (
	EndCompleted          = "completed"
	EndNoAction           = "no_action"
	EndInterrupted        = "interrupted"
	EndNoChoice           = "no_choice"
	EndTeammateReplied    = "teammate_replied"
	EndConversationClosed = "conversation_closed"
	EndConversationSpam   = "conversation_spam"
	EndConversationGone   = "conversation_deleted"
	EndStepBudget         = "step_budget"
	EndDefinitionError    = "definition_error"
	EndInternalError      = "internal_error"
	EndExpired            = "expired"
)

/*
Step names what to advance: a run, and either the sequence a job
expects or what a waiting run receives.
*/
type Step struct {
	WorkspaceID ids.UUID
	RunID       ids.UUID
	Seq         *int64
	Input       *Input
}

/*
Input is what a waiting run receives: a customer message, a teammate's
reply, its timer, or the agent handing the conversation over, with why.
*/
type Input struct {
	Kind      string    `json:"kind"`
	MessageID *ids.UUID `json:"message_id,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Text      string    `json:"-"`
}

/*
The input kinds.
*/
const (
	InputMessage       = "message"
	InputTimer         = "timer"
	InputTeammateReply = "teammate_reply"
	InputAgentHandover = "agent_handover"
)

/*
Advance moves a run on as far as it goes now.
*/
func (e *Engine) Advance(ctx context.Context, s Step) error {
	return e.transaction(ctx, s.WorkspaceID, func(ctx context.Context, tx *database.Tx) error {
		st, err := e.open(ctx, tx, s)
		if err != nil || st == nil {
			return err
		}

		if err := st.loop(ctx); err != nil {
			return err
		}

		return st.save(ctx)
	})
}

/*
open locks the run's conversation and the run, and returns its state
when the step still applies to it.
*/
func (e *Engine) open(ctx context.Context, tx *database.Tx, s Step) (*state, error) {
	q := models.New(tx)
	peek, err := q.FindWorkflowRun(ctx, models.FindWorkflowRunParams{WorkspaceID: s.WorkspaceID, ID: s.RunID})
	if errors.Is(err, database.ErrNotFound) || err == nil && peek.ConversationID == nil {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find workflow run: %w", err)
	}

	if err := tx.AdvisoryLock(ctx, "wf:conv:"+peek.ConversationID.String()); err != nil {
		return nil, fmt.Errorf("lock conversation runs: %w", err)
	}

	run, err := q.LockWorkflowRun(ctx, models.LockWorkflowRunParams{WorkspaceID: s.WorkspaceID, ID: s.RunID})
	if err != nil {
		return nil, fmt.Errorf("lock workflow run: %w", err)
	}

	if !e.applies(run, s) {
		return nil, nil
	}

	return e.load(ctx, tx, run, s.Input)
}

/*
applies reports whether the step still applies: a continuation to a
running run at its sequence, an input to a run waiting for that kind,
a timer only once it is due.
*/
func (e *Engine) applies(run models.WorkflowRun, s Step) bool {
	if s.Seq != nil && *s.Seq != run.Seq {
		return false
	}

	if s.Input == nil {
		return run.Status == StatusRunning
	}

	if run.Status != StatusWaiting {
		return false
	}

	w := readWait(run.Wait)
	switch s.Input.Kind {
	case InputTimer:
		return run.WakeAt != nil && !run.WakeAt.After(e.Clock.Now())
	case InputMessage:
		return w.Kind == waitInput || w.Kind == workflowspec.WaitAgent || w.interrupts(interruptCustomer)
	case InputTeammateReply:
		return w.interrupts(interruptTeammate)
	case InputAgentHandover:
		return w.Kind == workflowspec.WaitAgent
	}

	return false
}

/*
load reads everything a run needs to advance.
*/
func (e *Engine) load(ctx context.Context, tx *database.Tx, run models.WorkflowRun, in *Input) (*state, error) {
	q := models.New(tx)
	version, err := e.version(ctx, q, run)
	if err != nil {
		return nil, err
	}

	wf, err := q.FindWorkflow(ctx, models.FindWorkflowParams{WorkspaceID: run.WorkspaceID, ID: run.WorkflowID})
	if err != nil {
		return nil, fmt.Errorf("find workflow: %w", err)
	}

	conv, err := q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: run.WorkspaceID, ID: *run.ConversationID,
	})
	if err != nil {
		return nil, fmt.Errorf("find conversation: %w", err)
	}

	st := &state{
		e: e, tx: tx, q: q, run: run, version: version, name: wf.Name, conv: conv, input: in,
		vars: map[string]any{}, cursor: deref(run.CursorStepID), wait: readWait(run.Wait), acted: run.Acted,
	}
	_ = json.Unmarshal(run.Vars, &st.vars)

	if in != nil && in.MessageID != nil {
		st.loadInputText(ctx)
	}

	return st, nil
}

/*
state is a run while it advances.
*/
type state struct {
	e        *Engine
	tx       *database.Tx
	q        *models.Queries
	run      models.WorkflowRun
	version  candidate
	name     string
	conv     models.Conversation
	subj     *subject
	vars     map[string]any
	cursor   string
	wait     waitState
	input    *Input
	acted    bool
	steps    int32
	status   string
	resuming bool
	reason   string
	failure  json.RawMessage
}

/*
loop executes steps until the run waits, ends or did its share.
*/
func (st *state) loop(ctx context.Context) error {
	st.status = StatusRunning
	if st.input != nil {
		st.status = StatusWaiting
		if err := st.resume(ctx); err != nil || st.status != StatusRunning {
			return err
		}
	}

	for range st.e.Limits.StepsPerJob {
		if st.cursor == "" {
			st.finish(ctx, EndCompleted)
			return nil
		}

		if int(st.run.StepsExecuted+st.steps) >= st.e.Limits.StepsPerRun {
			st.fail(ctx, EndStepBudget, "The run executed its most steps.")
			return nil
		}

		if err := st.execute(ctx); err != nil || st.status != StatusRunning {
			return err
		}
	}

	return nil
}

/*
execute runs the step under the cursor.
*/
func (st *state) execute(ctx context.Context) error {
	step, ok := st.version.index.Step(st.cursor)
	if !ok {
		st.fail(ctx, EndDefinitionError, "The run's step is not in its version.")
		return nil
	}

	exec, ok := executors()[step.Kind]
	if !ok {
		st.fail(ctx, EndDefinitionError, "This step cannot run.")
		return nil
	}

	st.steps++
	out, err := exec(ctx, st, step)
	if err != nil {
		return fmt.Errorf("run step %s: %w", step.ID, err)
	}

	return st.apply(ctx, step, out)
}

/*
resume hands the waiting step what it waited for.
*/
func (st *state) resume(ctx context.Context) error {
	step, ok := st.version.index.Step(st.cursor)
	if !ok {
		st.fail(ctx, EndDefinitionError, "The run's step is not in its version.")
		return nil
	}

	res, ok := resumers()[step.Kind]
	if !ok {
		st.fail(ctx, EndDefinitionError, "This step cannot be resumed.")
		return nil
	}

	st.status = StatusRunning
	out, err := res(ctx, st, step, *st.input)
	if err != nil {
		return fmt.Errorf("resume step %s: %w", step.ID, err)
	}

	st.input, st.resuming = nil, true
	err = st.apply(ctx, step, out)
	st.resuming = false

	return err
}

/*
apply records the step's trace and moves the run as the outcome says.
*/
func (st *state) apply(ctx context.Context, step workflowspec.Step, out outcome) error {
	if err := st.trace(ctx, step, out); err != nil {
		return err
	}

	st.acted = st.acted || out.acted
	switch {
	case out.wait != nil:
		out.wait.StepID = step.ID
		st.wait, st.status = *out.wait, StatusWaiting
	case out.end != "":
		st.finish(ctx, out.end)
	case out.path != "":
		st.cursor = st.version.index.Enter(step.ID, out.path)
	default:
		st.cursor = st.version.index.Next(step.ID)
	}

	return nil
}

/*
deref returns the text, empty for none.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

/*
writer returns a writer acting for the workflow, whose changes carry
the chain that stops workflows from triggering themselves.
*/
func (st *state) writer() *conversations.Writer {
	chain := append(append([]ids.UUID{}, st.run.Chain...), st.run.WorkflowID)

	return &conversations.Writer{
		Q: st.q, Tx: st.tx, Events: st.e.Events, Now: st.e.Clock.Now(), Chain: chain,
		Actor: conversations.Actor{Type: string(enums.AuthorTypeSystem), Name: "Workflow: " + st.name},
	}
}

/*
subject returns the conversation's condition facts, loaded once.
*/
func (st *state) subject(ctx context.Context) (*subject, error) {
	if st.subj != nil {
		st.subj.conversation, st.subj.vars, st.subj.now = st.conv, st.vars, st.e.Clock.Now()
		return st.subj, nil
	}

	s, err := loadSubject(ctx, st.q, st.conv)
	if err != nil {
		return nil, err
	}

	if err := st.e.withHours(ctx, st.q, s); err != nil {
		return nil, err
	}

	_ = json.Unmarshal(st.run.TriggerData, &s.trigger)
	s.vars, s.now = st.vars, st.e.Clock.Now()
	st.subj = s

	return s, nil
}

/*
now is the time of the step.
*/
func (st *state) now() time.Time {
	return st.e.Clock.Now()
}
