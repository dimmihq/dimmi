/*
|--------------------------------------------------------------------------
| Agent Readiness
|--------------------------------------------------------------------------
|
| A workflow may let dimmi handle conversations before the workspace's
| agent is switched on: publishing is allowed, since the step hands the
| conversation over at once while the agent is off, but the publisher
| is warned so the customer is not handed over by surprise.
|
*/

package workflows

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
agentWarnings warns when the definition lets dimmi handle conversations
and the workspace's agent is missing or off.
*/
func agentWarnings(ctx context.Context, q *models.Queries, wf models.Workflow, spec workflowspec.Spec,
) ([]workflowspec.Issue, error) {
	step, ok := firstOfKind(spec.Steps, workflowspec.KindDimmi)
	if !ok {
		return nil, nil
	}

	agent, err := q.FindAgent(ctx, wf.WorkspaceID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("find agent: %w", err)
	}

	if err == nil && agent.Enabled {
		return nil, nil
	}

	return []workflowspec.Issue{{
		Code: workflowspec.WarnDimmiNotReady, StepID: step.ID,
		Message: "dimmi is off in this workspace, so this step hands every conversation to the team.",
	}}, nil
}

/*
firstOfKind returns the first step of the kind anywhere in the tree.
*/
func firstOfKind(steps []workflowspec.Step, kind string) (workflowspec.Step, bool) {
	for _, s := range steps {
		if s.Kind == kind {
			return s, true
		}

		for _, p := range s.Paths {
			if found, ok := firstOfKind(p.Steps, kind); ok {
				return found, true
			}
		}
	}

	return workflowspec.Step{}, false
}
