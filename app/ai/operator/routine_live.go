/*
|--------------------------------------------------------------------------
| Watching A Routine Run
|--------------------------------------------------------------------------
|
| A routine's run belongs to no one teammate, so it streams on the
| routine's own channel, which every teammate who may read the routine
| may follow: its text as the model writes it, each step as it finishes,
| and each change of the run's status. Each finished step is also kept
| on the run, so a teammate who opens the run midway sees what it did so
| far before the rest streams in.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"fmt"

	fbroadcasting "github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EventRunChanged tells a routine's watchers that one of its runs started,
finished or was skipped.
*/
const EventRunChanged = "operator.routine_run.changed"

/*
keepSteps saves the steps a routine's run has finished so far. A save
that fails only costs a late watcher the steps; the run goes on.
*/
func (e *Engine) keepSteps(ctx context.Context, s *session) {
	if s.routine == nil {
		return
	}
	encoded, err := json.Marshal(stepsOrEmpty(s.steps))
	if err != nil {
		return
	}
	run := s.routine.run
	saveErr := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		err := models.New(tx).SaveOperatorRoutineRunSteps(ctx, models.SaveOperatorRoutineRunStepsParams{
			WorkspaceID: run.WorkspaceID, ID: run.ID, Steps: encoded, Now: e.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("keep routine run steps: %w", err)
		}
		return nil
	})
	unsaved(ctx, "routine_save_failed", saveErr)
}

/*
publishRun tells the routine's watchers how one of its runs now stands.
*/
func (e *Engine) publishRun(ctx context.Context, run models.OperatorRoutineRun) {
	if e.Publisher == nil {
		return
	}
	data := map[string]any{
		"routine_id": models.PublicID(models.PrefixOperatorRoutine, run.RoutineID),
		"run_id":     models.PublicID(models.PrefixOperatorRoutineRun, run.ID),
		"status":     run.Status,
	}
	if run.ThreadID != nil {
		data["thread_id"] = models.PublicID(models.PrefixOperatorThread, *run.ThreadID)
	}
	saveErr := e.Publisher.Publish(ctx, []string{appbroadcasting.OperatorRoutineChannel(run.RoutineID)},
		fbroadcasting.Message{ID: ids.Encode("evt", ids.New()), Event: EventRunChanged, Data: data})
	unsaved(ctx, "routine_publish_failed", saveErr)
}
