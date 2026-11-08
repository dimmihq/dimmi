/*
|--------------------------------------------------------------------------
| Workflow Control
|--------------------------------------------------------------------------
|
| A conversation a workflow holds hears from the agent only while one
| of the workflow's steps lets dimmi handle it. That step's settings
| live on the run waiting on it, and every turn reads them there: extra
| guidance, a reply limit and whether answers ask the customer to
| confirm. Inside such a step the workflow routes a handoff down its
| own path, so the agent's own handoff routing stays out of it. A
| customer confirming an answer is the workflow's to act on, never a
| new question for the agent.
|
*/

package pipeline

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/resolution"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
loadWorkflowStep reads the active agent step of the run holding the
conversation, if there is one.
*/
func loadWorkflowStep(ctx context.Context, q *models.Queries, e *env) error {
	c := e.Conversation
	if c.AiMode != "workflow" || c.WorkflowRunID == nil {
		return nil
	}

	run, err := q.FindWorkflowRun(ctx, models.FindWorkflowRunParams{WorkspaceID: c.WorkspaceID, ID: *c.WorkflowRunID})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find workflow run: %w", err)
	}

	if run.Status != "waiting" {
		return nil
	}

	if step, ok := workflowspec.AgentHandlingOf(run.Wait); ok {
		e.WorkflowStep = &step
	}

	return nil
}

/*
applyWorkflowStep makes the turn follow the active agent step.
*/
func applyWorkflowStep(e *env, s *TurnSettings) {
	step := e.WorkflowStep
	if step == nil {
		return
	}

	s.Workflow.StepActive = true
	s.Instructions = step.Instructions
	s.ConfirmResolution = step.ConfirmResolution
	s.HandoffTeamID, s.RaisePriority = nil, false
	if step.MaxReplies > 0 {
		s.MaxReplies = int32(min(step.MaxReplies, 50))
	}
}

/*
confirmsAnswer reports whether the pending message confirms the
agent's answer right before it, which the workflow resolves.
*/
func confirmsAnswer(e *env, s *TurnSettings) bool {
	return s.Workflow.StepActive && s.ConfirmResolution &&
		e.ReplyAfter == string(enums.AuthorTypeAgent) && resolution.Confirms(e.pendingText())
}

/*
withConfirmation ends an answer with the question that asks the
customer to confirm it, when the turn asks for confirmation.
*/
func withConfirmation(text string, st *run, d *decision) string {
	if !st.settings.ConfirmResolution || d.outcome != outcomeAnswered {
		return text
	}

	return text + "\n\n" + resolution.Prompt
}
