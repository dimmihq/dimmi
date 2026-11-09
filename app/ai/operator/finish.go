/*
|--------------------------------------------------------------------------
| Finishing A Turn
|--------------------------------------------------------------------------
|
| The answer is stored with the steps that led to it, even when the run
| was stopped or failed, so the thread shows what happened. The changes
| the run proposed are tied to the answer, and the run's cost lands on
| the month in the same transaction, so the budget is never behind.
|
*/

package operator

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The answers a run writes when the model wrote nothing.
*/
const (
	stoppedText = "Stopped."
	failedText  = "Something went wrong while I was working on this. Try again in a moment."
)

/*
finish stores the answer, ties the proposed changes to it and records
the run's cost.
*/
func (e *Engine) finish(ctx context.Context, s *session, text, status string) (Answer, error) {
	if text == "" {
		text = map[string]string{"stopped": stoppedText, "failed": failedText}[status]
	}
	var out Answer
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		msg, err := e.store(ctx, q, s, said{
			role: roleAssistant, body: text, status: status, steps: s.steps, run: &s.run.ID,
		})
		if err != nil {
			return err
		}
		out.Message = msg
		if err := e.attach(ctx, q, s, msg.ID); err != nil {
			return err
		}
		if err := e.finishRun(ctx, q, s, status); err != nil {
			return err
		}
		return addSpend(ctx, tx, e, s)
	})
	if err != nil {
		return out, fmt.Errorf("finish operator turn: %w", err)
	}
	out.Changesets, err = e.reload(ctx, s)
	e.publishAnswered(ctx, s, out.Message)
	return out, err
}

/*
attach ties the changesets the run proposed to its answer.
*/
func (e *Engine) attach(ctx context.Context, q *models.Queries, s *session, message ids.UUID) error {
	if len(s.proposed) == 0 {
		return nil
	}
	list := make([]ids.UUID, len(s.proposed))
	for i, c := range s.proposed {
		list[i] = c.ID
	}
	err := q.AttachOperatorChangesets(ctx, models.AttachOperatorChangesetsParams{
		WorkspaceID: s.m.Workspace.ID, MessageID: &message, Ids: list,
	})
	if err != nil {
		return fmt.Errorf("attach operator changesets: %w", err)
	}
	return nil
}

/*
reload reads the run's changesets as they now stand, applied or not.
*/
func (e *Engine) reload(ctx context.Context, s *session) ([]models.OperatorChangeset, error) {
	out := make([]models.OperatorChangeset, 0, len(s.proposed))
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		for _, c := range s.proposed {
			row, err := q.FindOperatorChangeset(ctx, models.FindOperatorChangesetParams{
				WorkspaceID: s.m.Workspace.ID, ID: c.ID,
			})
			if err != nil {
				return fmt.Errorf("find operator changeset: %w", err)
			}
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

/*
addSpend adds the run's cost and tokens to the month.
*/
func addSpend(ctx context.Context, tx *database.Tx, e *Engine, s *session) error {
	return pipeline.AddTaskSpend(ctx, tx, s.agent, pipeline.TaskSpend{
		At: e.Clock.Now(), Cost: s.tally.cost, Input: s.tally.input, Cached: s.tally.cached, Output: s.tally.output,
	})
}
