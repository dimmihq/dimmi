/*
|--------------------------------------------------------------------------
| Update Agent
|--------------------------------------------------------------------------
|
| One save of an agent's working copy: identity, guidance, models,
| rules, channels, schedule, handoff, privacy, what it may search,
| follow and call, which agents it may hand to and where it answers.
| Only what the caller sends changes; a field it leaves out keeps its
| value, and clearing a nullable field is an explicit request. Every
| connection, team, tag, agent and channel it names must belong to the
| workspace. Nothing reaches customers until the agent is published; a
| draft edited back to what is published is dropped.
|
*/

package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Optional is a patch field that may be left out, set or cleared: Set
says the caller sent it, and a nil Value then clears the stored value.
*/
type Optional[T any] struct {
	Set   bool
	Value *T
}

/*
AgentPatch is what one save changes. Plain pointer fields change when
non-nil and cannot be cleared; Optional fields may also be cleared.
EnabledChannels, Languages and LinkAllowlist replace their lists
whole; link hosts are stored lowercase. HandoffTeamID names the team a
handed-off conversation goes to, HandoffMode the agent's role and
AlwaysHandoffTags the tags that always send a conversation to a
person; that list replaces the stored one whole. Knowledge, Procedures
and Actions replace what the agent may use; TransferAgentIDs the
agents it may hand to; Placement where it answers.
*/
type AgentPatch struct {
	Name, Tone, Guidance, Schedule     *string
	Description                        *string
	AssignedBehavior, UnknownBehavior  *string
	DefaultLanguage, IncludeContact    *string
	RequireCitations                   *bool
	PiiRedaction, Streaming, RaisePrio *bool
	EnabledChannels, Languages         *[]string
	LinkAllowlist                      *[]string
	MaxAiReplies                       *int32
	ProductName, AnswerModel           Optional[string]
	FallbackModel                      Optional[string]
	AnswerKeyID, FallbackKeyID         Optional[ids.UUID]
	HandoffMode                        *enums.AIHandoffMode
	AlwaysHandoffTags                  *[]ids.UUID
	HandoffTeamID                      Optional[ids.UUID]
	Knowledge                          *agents.KnowledgeScope
	Procedures, Actions                *agents.Scope
	TransferAgentIDs                   *[]ids.UUID
	Placement                          *agents.Placement
}

/*
UpdateAgentInput is who saves, which agent, what changes and the lock
version the editor read; zero skips the check.
*/
type UpdateAgentInput struct {
	Membership  authz.Membership
	AgentID     ids.UUID
	Patch       AgentPatch
	LockVersion int32
}

/*
Update saves the agent's working copy.

Errors: [ErrAgentNotFound], [ErrAgentChanged], a [fault] naming the
field it refuses, or a wrapped database error.
*/
func (a *Agents) Update(ctx context.Context, in UpdateAgentInput) (*AgentView, error) {
	if err := RequireAgentManage(in.Membership); err != nil {
		return nil, err
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
		if in.LockVersion != 0 && in.LockVersion != agent.LockVersion {
			return ErrAgentChanged
		}
		saved, err := a.saveDraft(ctx, tx, agent, in)
		if err != nil {
			return err
		}
		view, err = a.view(ctx, tx, saved, key != nil)
		return err
	})
	return view, err
}

/*
saveDraft folds the patch into the agent's working copy and keeps it,
dropping a draft that says what is published already.
*/
func (a *Agents) saveDraft(ctx context.Context, tx *database.Tx, agent models.AiAgent, in UpdateAgentInput,
) (models.AiAgent, error) {
	q := models.New(tx)
	placements, err := agents.Placements(ctx, q, agent.WorkspaceID)
	if err != nil {
		return agent, err
	}
	working, err := agents.Working(agent, placements[agent.ID])
	if err != nil {
		return agent, err
	}

	next := patched(agent, working, in.Patch)
	next.Name = strings.TrimSpace(next.Name)
	err = checkName(ctx, q, models.AgentNameTakenParams{WorkspaceID: agent.WorkspaceID, ID: agent.ID, Name: next.Name})
	if err != nil {
		return agent, err
	}
	if err := checkPatch(ctx, tx, agent, in.Patch); err != nil {
		return agent, err
	}

	var draft json.RawMessage
	if !agents.Same(next, agents.Published(agent, placements[agent.ID])) || agent.Status == "draft" {
		draft, _ = json.Marshal(next)
	}
	saved, err := q.SaveAgentDraft(ctx, models.SaveAgentDraftParams{
		WorkspaceID: agent.WorkspaceID, ID: agent.ID, Name: publishedName(agent, next, draft),
		Draft: draft, MemberID: memberOf(in.Membership), LockVersion: agent.LockVersion, Now: a.Clock.Now(),
	})
	if errors.Is(err, database.ErrNotFound) {
		return agent, ErrAgentChanged
	}
	if err != nil {
		return agent, fmt.Errorf("save agent draft: %w", err)
	}
	return saved, nil
}

/*
publishedName is the name the list shows: a never published agent
shows its working name, a published one keeps its published name until
it is published again.
*/
func publishedName(agent models.AiAgent, next agents.Definition, draft json.RawMessage) string {
	if agent.Status == "draft" || draft == nil {
		return next.Name
	}
	return agent.Name
}

/*
patched folds the patch into the working copy.
*/
func patched(agent models.AiAgent, working agents.Definition, p AgentPatch) agents.Definition {
	row := agent
	agents.Apply(&row, working)
	applyPatch(&row, p)
	next := agents.Published(row, working.Placement)
	if p.Knowledge != nil {
		next.Knowledge = *p.Knowledge
	}
	if p.Procedures != nil {
		next.Procedures = *p.Procedures
	}
	if p.Actions != nil {
		next.Actions = *p.Actions
	}
	if p.TransferAgentIDs != nil {
		next.TransferAgentIDs = distinct(*p.TransferAgentIDs)
	}
	if p.Placement != nil {
		next.Placement = agents.Placement{
			ChannelConnectionIDs: distinct(p.Placement.ChannelConnectionIDs),
			ChannelRoomIDs:       distinct(p.Placement.ChannelRoomIDs),
			InboxIDs:             distinct(p.Placement.InboxIDs),
		}
	}
	// Normalise scopes the way the published columns hold them.
	agents.Apply(&row, next)
	return agents.Published(row, next.Placement)
}

/*
applyPatch folds every field the caller sent into the agent row.
*/
func applyPatch(agent *models.AiAgent, p AgentPatch) {
	set(&agent.Name, p.Name)
	set(&agent.Tone, p.Tone)
	set(&agent.Guidance, p.Guidance)
	if p.Description != nil {
		agent.Description = strings.TrimSpace(*p.Description)
	}
	set(&agent.Schedule, p.Schedule)
	set(&agent.AssignedBehavior, p.AssignedBehavior)
	set(&agent.UnknownBehavior, p.UnknownBehavior)
	set(&agent.DefaultLanguage, p.DefaultLanguage)
	set(&agent.IncludeContactContext, p.IncludeContact)
	set(&agent.RequireCitations, p.RequireCitations)
	set(&agent.PiiRedaction, p.PiiRedaction)
	set(&agent.Streaming, p.Streaming)
	set(&agent.EnabledChannels, p.EnabledChannels)
	set(&agent.Languages, p.Languages)
	set(&agent.LinkAllowlist, lowered(p.LinkAllowlist))
	set(&agent.MaxAiReplies, p.MaxAiReplies)
	setText(&agent.ProductName, p.ProductName)
	setText(&agent.AnswerModel, p.AnswerModel)
	setText(&agent.FallbackModel, p.FallbackModel)
	setOptional(&agent.AnswerKeyID, p.AnswerKeyID)
	setOptional(&agent.FallbackKeyID, p.FallbackKeyID)
	agent.Handoff = mergeHandoff(agent.Handoff, p)
}

/*
set applies a field the caller sent.
*/
func set[T any](dst, src *T) {
	if src != nil {
		*dst = *src
	}
}

/*
lowered returns the list in lowercase, nil staying nil.
*/
func lowered(list *[]string) *[]string {
	if list == nil {
		return nil
	}
	out := make([]string, len(*list))
	for i, v := range *list {
		out[i] = strings.ToLower(v)
	}
	return &out
}

/*
setOptional applies a field the caller sent or cleared.
*/
func setOptional[T any](dst **T, src Optional[T]) {
	if src.Set {
		*dst = src.Value
	}
}

/*
setText applies a nullable text field, where an empty string clears it
like null does.
*/
func setText(dst **string, src Optional[string]) {
	if src.Set && src.Value != nil && *src.Value == "" {
		src.Value = nil
	}
	setOptional(dst, src)
}

/*
checkPatch refuses a save that names rows outside the workspace.
*/
func checkPatch(ctx context.Context, tx *database.Tx, agent models.AiAgent, p AgentPatch) error {
	roles := []struct {
		field string
		id    Optional[ids.UUID]
	}{
		{"answer_key_id", p.AnswerKeyID},
		{"fallback_key_id", p.FallbackKeyID},
	}
	for _, role := range roles {
		if err := checkKey(ctx, tx, agent.WorkspaceID, role.id); err != nil {
			return fault.New(fault.Invalid, "provider_key_not_found", "the connection does not exist").
				OnField(role.field)
		}
	}

	if err := checkTeam(ctx, tx, agent.WorkspaceID, p.HandoffTeamID); err != nil {
		return err
	}
	if err := checkTags(ctx, tx, agent.WorkspaceID, p.AlwaysHandoffTags); err != nil {
		return err
	}
	if err := checkTransfers(ctx, models.New(tx), agent, p.TransferAgentIDs); err != nil {
		return err
	}
	return checkScopes(ctx, models.New(tx), agent.WorkspaceID, p)
}

/*
checkKey finds the connection a role names, when the caller named one.
*/
func checkKey(ctx context.Context, tx *database.Tx, workspaceID ids.UUID, id Optional[ids.UUID]) error {
	if !id.Set || id.Value == nil {
		return nil
	}
	_, err := findKey(ctx, tx, workspaceID, *id.Value)
	return err
}

/*
checkTeam finds the handoff team, when the caller named one.
*/
func checkTeam(ctx context.Context, tx *database.Tx, workspaceID ids.UUID, id Optional[ids.UUID]) error {
	if !id.Set || id.Value == nil {
		return nil
	}

	_, err := models.New(tx).FindTeam(ctx, models.FindTeamParams{WorkspaceID: workspaceID, ID: *id.Value})
	if err != nil {
		return fault.New(fault.Invalid, "team_not_found", "the team does not exist").OnField("handoff_team_id")
	}
	return nil
}

/*
checkTransfers refuses handing to the agent itself or to an agent the
workspace does not have.
*/
func checkTransfers(ctx context.Context, q *models.Queries, agent models.AiAgent, list *[]ids.UUID) error {
	if list == nil || len(*list) == 0 {
		return nil
	}
	want := distinct(*list)
	if slices.Contains(want, agent.ID) {
		return fault.New(fault.Invalid, "transfer_to_self", "an agent cannot hand to itself").
			OnField("transfer_agent_ids")
	}
	found, err := q.ListAgentsByID(ctx, models.ListAgentsByIDParams{WorkspaceID: agent.WorkspaceID, Ids: want})
	if err != nil {
		return fmt.Errorf("find transfer agents: %w", err)
	}
	if len(found) != len(want) {
		return fault.New(fault.Invalid, "agent_not_found", "an agent it hands to does not exist").
			OnField("transfer_agent_ids")
	}
	return nil
}
