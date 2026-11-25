/*
|--------------------------------------------------------------------------
| Agent Ties
|--------------------------------------------------------------------------
|
| What still relies on an agent: workflows and series that name it, the
| places it answers on, and the agents that may transfer to it. An agent
| is archived only once nothing does.
|
*/

package ai

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrAgentTied refuses to archive an agent something still relies on.
Its meta lists each tie by kind, id and name.
*/
var ErrAgentTied = fault.New(fault.Conflict, "agent_in_use",
	"other parts of the workspace still use this agent; move them to another agent first")

/*
Tie is one thing that still relies on an agent: a workflow, a series, a
place it answers on (connection, room or inbox) or another agent that
may transfer to it.
*/
type Tie struct {
	Kind string
	ID   ids.UUID
	Name string
}

/*
Ties lists what still relies on the agent.
*/
func (a *Agents) Ties(ctx context.Context, m authz.Membership, id ids.UUID) ([]Tie, error) {
	q := models.New(a.DB)
	if _, err := findAgent(ctx, q, m.Workspace.ID, id); err != nil {
		return nil, err
	}
	return agentTies(ctx, q, m.Workspace.ID, id)
}

/*
agentTies reads the agent's ties.
*/
func agentTies(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) ([]Tie, error) {
	rows, err := q.AgentTies(ctx, models.AgentTiesParams{
		WorkspaceID: workspaceID, AgentID: &id, PublicID: models.PublicID(models.PrefixAiAgent, id),
	})
	if err != nil {
		return nil, fmt.Errorf("list agent ties: %w", err)
	}
	ties := make([]Tie, 0, len(rows))
	for _, r := range rows {
		ties = append(ties, Tie{Kind: r.Kind, ID: r.ID, Name: r.Name})
	}
	return ties, nil
}

/*
tieMeta writes ties for an error's meta, with public ids.
*/
func tieMeta(ties []Tie) []map[string]string {
	out := make([]map[string]string, 0, len(ties))
	for _, t := range ties {
		out = append(out, map[string]string{"kind": t.Kind, "id": TiePublicID(t), "name": t.Name})
	}
	return out
}

/*
TiePublicID is the public id of what a tie names.
*/
func TiePublicID(t Tie) string {
	switch t.Kind {
	case "workflow":
		return models.PublicID(models.PrefixWorkflow, t.ID)
	case "series":
		return models.PublicID(models.PrefixSeries, t.ID)
	case "connection":
		return models.PublicID(models.PrefixChannelConnection, t.ID)
	case "room":
		return models.PublicID(models.PrefixChannelRoom, t.ID)
	case "inbox":
		return models.PublicID(models.PrefixInbox, t.ID)
	case "agent":
		return models.PublicID(models.PrefixAiAgent, t.ID)
	}
	return t.ID.String()
}
