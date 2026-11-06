/*
|--------------------------------------------------------------------------
| Agent Runs
|--------------------------------------------------------------------------
|
| The trace behind a conversation's "why": every run the agent made on
| it and, for one run, each step it took. Anyone who may see the
| conversation sees the numbers; the redacted prompts and replies the
| steps carry are for the people allowed to read payloads.
|
*/

package ai

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
runsPageLimit is how many runs one conversation's list shows.
*/
const runsPageLimit = 100

/*
ListRunsInput names the conversation whose runs to list.
*/
type ListRunsInput struct {
	Membership     authz.Membership
	ConversationID ids.UUID
}

/*
ListConversationRuns returns a conversation's runs, newest first.

Rules:
  - The caller needs ai:read and must be able to see the conversation.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ListConversationRuns struct {
	DB *database.DB
}

/*
Handle lists the runs.
*/
func (a *ListConversationRuns) Handle(ctx context.Context, in ListRunsInput) ([]models.AiRun, error) {
	if err := RequireAgentRead(in.Membership); err != nil {
		return nil, err
	}

	var runs []models.AiRun

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := conversations.Find(ctx, q, in.Membership, conversations.Ref{ID: in.ConversationID}); err != nil {
			return err
		}

		var err error
		runs, err = q.ListConversationRuns(ctx, models.ListConversationRunsParams{
			WorkspaceID: in.Membership.Workspace.ID, ConversationID: &in.ConversationID, PageLimit: runsPageLimit,
		})
		if err != nil {
			return fmt.Errorf("list runs: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return runs, nil
}

/*
ShowRunInput names one run of a conversation.
*/
type ShowRunInput struct {
	Membership     authz.Membership
	ConversationID ids.UUID
	RunID          ids.UUID
}

/*
RunView is one run with its steps, in order. Payloads says whether the
steps still carry their request and response; without the permission
they never do.
*/
type RunView struct {
	Run      models.AiRun
	Steps    []models.AiRunStep
	Payloads bool
}

/*
ShowRun returns one run of a conversation with its steps.

Rules:
  - The caller needs ai:read and must be able to see the conversation;
    a run of another conversation is not found.
  - Step payloads are left out unless the caller holds
    ai_runs:read_payloads.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowRun struct {
	DB *database.DB
}

/*
Handle returns the run.
*/
func (a *ShowRun) Handle(ctx context.Context, in ShowRunInput) (*RunView, error) {
	if err := RequireAgentRead(in.Membership); err != nil {
		return nil, err
	}

	view := &RunView{Payloads: CanReadPayloads(in.Membership)}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := conversations.Find(ctx, q, in.Membership, conversations.Ref{ID: in.ConversationID}); err != nil {
			return err
		}

		run, err := q.FindRun(ctx, models.FindRunParams{WorkspaceID: in.Membership.Workspace.ID, ID: in.RunID})
		if err != nil || run.ConversationID == nil || *run.ConversationID != in.ConversationID {
			return exceptions.ErrNotFound
		}
		view.Run = run

		view.Steps, err = q.ListRunSteps(ctx, models.ListRunStepsParams{
			WorkspaceID: in.Membership.Workspace.ID, RunID: run.ID,
		})
		if err != nil {
			return fmt.Errorf("list run steps: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !view.Payloads {
		for i := range view.Steps {
			view.Steps[i].Request, view.Steps[i].Response = nil, nil
		}
	}

	return view, nil
}
