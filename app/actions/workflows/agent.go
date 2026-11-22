/*
|--------------------------------------------------------------------------
| Agent Readiness
|--------------------------------------------------------------------------
|
| A workflow may let dimmi handle conversations before the agent that
| would answer is switched on: publishing is allowed, since the step hands the
| conversation over at once while the agent is off, but the publisher
| is warned so the customer is not handed over by surprise.
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

	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
agentWarnings warns when the definition lets dimmi handle conversations
and the workspace's agent is missing or off.
*/
func (w *Workflows) agentWarnings(ctx context.Context, q *models.Queries, wf models.Workflow, spec workflowspec.Spec,
) (errs, warns []workflowspec.Issue, err error) {
	step, ok := firstOfKind(spec.Steps, workflowspec.KindDimmi)
	if !ok {
		return nil, nil, nil
	}

	var d workflowspec.DimmiSettings
	_ = json.Unmarshal(step.Settings, &d)
	where := agents.Where{WorkspaceID: wf.WorkspaceID}
	named, found, err := w.stepAgent(ctx, q, wf, d.AgentID)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return []workflowspec.Issue{missingIssue("agent", d.AgentID)}, nil, nil
	}
	where.WorkflowAgentID = named

	agent, err := agents.Resolve(ctx, q, where)
	if err != nil && !errors.Is(err, agents.ErrNoAgent) {
		return nil, nil, err
	}

	if err == nil && agent.Enabled {
		return nil, nil, nil
	}

	return nil, []workflowspec.Issue{{
		Code: workflowspec.WarnDimmiNotReady, StepID: step.ID,
		Message: "The agent this step hands to is off, so it hands every conversation to the team.",
	}}, nil
}

/*
stepAgent decodes the agent a dimmi step names, reporting whether the
workspace has it; a step that names none finds nothing and needs none.
*/
func (w *Workflows) stepAgent(ctx context.Context, q *models.Queries, wf models.Workflow, public string,
) (*ids.UUID, bool, error) {
	if public == "" {
		return nil, true, nil
	}
	id, ok := w.decodeAgent(public)
	if !ok {
		return nil, false, nil
	}
	_, err := q.FindAgentByID(ctx, models.FindAgentByIDParams{WorkspaceID: wf.WorkspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("find agent: %w", err)
	}
	return &id, true, nil
}

/*
decodeAgent decodes an agent's public id, reporting whether it is one.
*/
func (w *Workflows) decodeAgent(public string) (ids.UUID, bool) {
	id, err := w.IDs.Decode(string(models.PrefixAiAgent), public)
	return id, err == nil
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
