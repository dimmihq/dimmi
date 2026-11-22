/*
|--------------------------------------------------------------------------
| Publish Agent
|--------------------------------------------------------------------------
|
| Publishing makes the working copy what customers get: its settings
| become the agent's, it takes the channels and inboxes it names, and a
| numbered version keeps it as it was, with a note on what changed.
| Turning an agent off stops it answering and keeps everything else. A
| version can be put back as the working copy, or published again.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ErrVersionNotFound says the agent has no such version.
*/
var ErrVersionNotFound = fault.New(fault.NotFound, "version_not_found", "the version does not exist")

/*
ErrAgentAlreadyOff says the agent is not answering, so there is
nothing to turn off.
*/
var ErrAgentAlreadyOff = fault.New(fault.Conflict, "agent_not_live", "the agent is not answering")

/*
PublishInput is who publishes which agent and why; ChangeNote is kept
on the version.
*/
type PublishInput struct {
	Membership authz.Membership
	AgentID    ids.UUID
	ChangeNote string
}

/*
Publish makes the agent's working copy live.

Errors: [ErrAgentNotFound], a [fault] saying why the agent could not
answer, or a wrapped database error.
*/
func (a *Agents) Publish(ctx context.Context, in PublishInput) (*AgentView, error) {
	if err := RequireAgentManage(in.Membership); err != nil {
		return nil, err
	}
	if len([]rune(in.ChangeNote)) > 500 {
		return nil, fault.New(fault.Invalid, "change_note_too_long", "keep the note to 500 characters").
			OnField("change_note")
	}
	var view *AgentView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, key, err := a.Provision(ctx, tx, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}
		agent, err := findAgent(ctx, models.New(tx), in.Membership.Workspace.ID, in.AgentID)
		if err != nil {
			return err
		}
		published, err := a.goLive(ctx, tx, agent, release{
			member: memberOf(in.Membership), note: strings.TrimSpace(in.ChangeNote),
		})
		if err != nil {
			return err
		}
		view, err = a.view(ctx, tx, published, key != nil)
		return err
	})
	return view, err
}

/*
release is who publishes and the note kept on the version.
*/
type release struct {
	member *ids.UUID
	note   string
}

/*
goLive writes the working copy live in the caller's transaction.
*/
func (a *Agents) goLive(ctx context.Context, tx *database.Tx, agent models.AiAgent, r release,
) (models.AiAgent, error) {
	member, note := r.member, r.note
	q := models.New(tx)
	working, err := readyWorkingCopy(ctx, q, agent)
	if err != nil {
		return agent, err
	}

	row := agent
	agents.Apply(&row, working)
	row.Enabled = true
	if _, err := q.UpdateAgent(ctx, agents.UpdateParams(&row, a.Clock.Now())); err != nil {
		return agent, fmt.Errorf("publish agent settings: %w", err)
	}
	if err := agents.Place(ctx, q, agent, working.Placement); err != nil {
		return agent, err
	}

	definition, _ := json.Marshal(working)
	var changeNote *string
	if note != "" {
		changeNote = &note
	}
	version, err := q.CreateAgentVersion(ctx, models.CreateAgentVersionParams{
		WorkspaceID: agent.WorkspaceID, AgentID: agent.ID, Definition: definition,
		ChangeNote: changeNote, MemberID: member, Now: a.Clock.Now(),
	})
	if err != nil {
		return agent, fmt.Errorf("keep agent version: %w", err)
	}
	now := a.Clock.Now()
	out, err := q.PublishAgent(ctx, models.PublishAgentParams{
		WorkspaceID: agent.WorkspaceID, ID: agent.ID, LiveVersionID: &version.ID, MemberID: member, Now: &now,
	})
	if err != nil {
		return agent, fmt.Errorf("publish agent: %w", err)
	}
	return out, nil
}

/*
readyWorkingCopy returns the agent's working copy, refusing one that
could not answer.
*/
func readyWorkingCopy(ctx context.Context, q *models.Queries, agent models.AiAgent) (agents.Definition, error) {
	placements, err := agents.Placements(ctx, q, agent.WorkspaceID)
	if err != nil {
		return agents.Definition{}, err
	}
	working, err := agents.Working(agent, placements[agent.ID])
	if err != nil {
		return working, err
	}
	settings, err := agents.Shared(ctx, q, agent.WorkspaceID)
	if err != nil {
		return working, err
	}
	keys, err := q.ListProviderKeys(ctx, agent.WorkspaceID)
	if err != nil {
		return working, fmt.Errorf("list provider keys: %w", err)
	}
	if ready, reasons := readiness(draftAgent(agent, working, settings), keys); !ready {
		return working, fault.New(fault.Invalid, "agent_not_ready", "the agent cannot answer yet").
			With("reasons", reasons)
	}
	return working, nil
}

/*
Pause stops the agent answering, keeping its settings and draft.

Errors: [ErrAgentNotFound], [ErrAgentAlreadyOff], or a wrapped database
error.
*/
func (a *Agents) Pause(ctx context.Context, m authz.Membership, id ids.UUID) (*AgentView, error) {
	if err := RequireAgentManage(m); err != nil {
		return nil, err
	}
	var view *AgentView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, key, err := a.Provision(ctx, tx, m.Workspace.ID)
		if err != nil {
			return err
		}
		q := models.New(tx)
		if _, err := findAgent(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}
		paused, err := q.PauseAgent(ctx, models.PauseAgentParams{
			WorkspaceID: m.Workspace.ID, ID: id, MemberID: memberOf(m), Now: a.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return ErrAgentAlreadyOff
		}
		if err != nil {
			return fmt.Errorf("pause agent: %w", err)
		}
		view, err = a.view(ctx, tx, paused, key != nil)
		return err
	})
	return view, err
}

/*
AgentVersion is one published version, read back as a definition.
*/
type AgentVersion struct {
	Version    models.AiAgentVersion
	Definition agents.Definition
}

/*
VersionPage asks for at most Limit versions, below version Before when
it is above zero.
*/
type VersionPage struct {
	Before, Limit int32
}

/*
Versions returns a page of an agent's versions, newest first.
*/
func (a *Agents) Versions(ctx context.Context, m authz.Membership, id ids.UUID, page VersionPage,
) ([]AgentVersion, error) {
	if err := RequireAgentRead(m); err != nil {
		return nil, err
	}
	var out []AgentVersion
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := findAgent(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}
		rows, err := q.ListAgentVersions(ctx, models.ListAgentVersionsParams{
			WorkspaceID: m.Workspace.ID, AgentID: id, Before: page.Before, RowLimit: page.Limit,
		})
		if err != nil {
			return fmt.Errorf("list agent versions: %w", err)
		}
		for _, v := range rows {
			out = append(out, readVersion(v))
		}
		return nil
	})
	return out, err
}

/*
Version returns one version of an agent by number.
*/
func (a *Agents) Version(ctx context.Context, m authz.Membership, id ids.UUID, number int32) (*AgentVersion, error) {
	if err := RequireAgentRead(m); err != nil {
		return nil, err
	}
	var out *AgentVersion
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		v, err := findVersion(ctx, models.New(tx), models.FindAgentVersionParams{
			WorkspaceID: m.Workspace.ID, AgentID: id, Version: number,
		})
		if err != nil {
			return err
		}
		read := readVersion(v)
		out = &read
		return nil
	})
	return out, err
}

/*
RestoreInput is who puts which version of an agent back, and whether
it goes live at once.
*/
type RestoreInput struct {
	Membership authz.Membership
	AgentID    ids.UUID
	Version    int32
	Publish    bool
}

/*
Restore makes a version the working copy, and publishes it when asked.
*/
func (a *Agents) Restore(ctx context.Context, in RestoreInput) (*AgentView, error) {
	if err := RequireAgentManage(in.Membership); err != nil {
		return nil, err
	}
	var view *AgentView
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, key, err := a.Provision(ctx, tx, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}
		q := models.New(tx)
		agent, err := findAgent(ctx, q, in.Membership.Workspace.ID, in.AgentID)
		if err != nil {
			return err
		}
		v, err := findVersion(ctx, q, models.FindAgentVersionParams{
			WorkspaceID: agent.WorkspaceID, AgentID: agent.ID, Version: in.Version,
		})
		if err != nil {
			return err
		}
		agent, err = q.SaveAgentDraft(ctx, models.SaveAgentDraftParams{
			WorkspaceID: agent.WorkspaceID, ID: agent.ID, Name: agent.Name, Draft: v.Definition,
			MemberID: memberOf(in.Membership), LockVersion: agent.LockVersion, Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("restore agent version: %w", err)
		}
		if in.Publish {
			note := fmt.Sprintf("Restored version %d", v.Version)
			if agent, err = a.goLive(ctx, tx, agent, release{member: memberOf(in.Membership), note: note}); err != nil {
				return err
			}
		}
		view, err = a.view(ctx, tx, agent, key != nil)
		return err
	})
	return view, err
}

/*
findVersion returns one version of one of the workspace's agents.
*/
func findVersion(ctx context.Context, q *models.Queries, p models.FindAgentVersionParams,
) (models.AiAgentVersion, error) {
	if _, err := findAgent(ctx, q, p.WorkspaceID, p.AgentID); err != nil {
		return models.AiAgentVersion{}, err
	}
	v, err := q.FindAgentVersion(ctx, p)
	if errors.Is(err, database.ErrNotFound) {
		return v, ErrVersionNotFound
	}
	if err != nil {
		return v, fmt.Errorf("find agent version: %w", err)
	}
	return v, nil
}

/*
readVersion reads a version's definition back.
*/
func readVersion(v models.AiAgentVersion) AgentVersion {
	var def agents.Definition
	_ = json.Unmarshal(v.Definition, &def)
	return AgentVersion{Version: v, Definition: def}
}
