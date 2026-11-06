/*
|--------------------------------------------------------------------------
| Update AI Agent
|--------------------------------------------------------------------------
|
| The agent's whole settings surface in one save: identity, models,
| rules, channels, schedule, handoff, privacy and spend. Only what the
| caller sends changes; a field it leaves out keeps its stored value,
| and clearing a nullable field is an explicit request. Every
| connection and team it names must belong to the workspace, and an
| agent is only switched on once it could actually answer.
|
*/

package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
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
whole; link hosts are stored lowercase.
MonthlyCapMicros cleared means no cap. HandoffTeamID names the team a
handed-off conversation goes to, HandoffMode the agent's role and
AlwaysHandoffTags the tags that always send a conversation to a
person; that list replaces the stored one whole.
*/
type AgentPatch struct {
	Name, Tone, Guidance, Schedule     *string
	AssignedBehavior, UnknownBehavior  *string
	DefaultLanguage, IncludeContact    *string
	Enabled, RequireCitations          *bool
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
	MonthlyCapMicros                   Optional[int64]
}

/*
UpdateAgentInput is who saves and what changes.
*/
type UpdateAgentInput struct {
	Membership authz.Membership
	Patch      AgentPatch
}

/*
UpdateAiAgent saves the agent's settings.

Rules:
  - The caller needs ai_agent:manage.
  - Connections named for a role, the handoff team and the tags that
    always hand off must be live rows of the workspace.
  - Switching the agent on needs a usable answer model and a channel.
  - Changing the cap lifts a pause the old cap caused, when the month's
    spend is below the new one.

Errors: [exceptions.ErrForbidden], a [fault] naming the field it
refuses, or a wrapped database error.
*/
type UpdateAiAgent struct {
	DB     *database.DB
	Clock  clock.Clock
	Config config.AIConfig
}

/*
Handle saves and returns the agent.
*/
func (a *UpdateAiAgent) Handle(ctx context.Context, in UpdateAgentInput) (*models.AiAgent, error) {
	if err := RequireAgentManage(in.Membership); err != nil {
		return nil, err
	}

	var out models.AiAgent

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		agent, err := ensureAgent(ctx, seedDeps{clock: a.Clock, config: a.Config}, tx, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}

		applyPatch(&agent, in.Patch)

		if err := checkPatch(ctx, tx, agent, in.Patch); err != nil {
			return err
		}

		out, err = models.New(tx).UpdateAgent(ctx, updateParams(&agent, a.Clock.Now()))
		if err != nil {
			return fmt.Errorf("update ai agent: %w", err)
		}

		return a.liftCap(ctx, tx, out, in.Patch)
	})
	if err != nil {
		return nil, err
	}

	return &out, nil
}

/*
liftCap clears the month's pause when the cap changed and the spend so
far sits below the new cap, or there is no cap any more.
*/
func (a *UpdateAiAgent) liftCap(ctx context.Context, tx *database.Tx, agent models.AiAgent, p AgentPatch) error {
	if !p.MonthlyCapMicros.Set {
		return nil
	}

	now := a.Clock.Now().UTC()
	start := database.Date(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC))

	period, err := models.New(tx).GetSpendPeriod(ctx, models.GetSpendPeriodParams{
		WorkspaceID: agent.WorkspaceID, PeriodStart: start,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find spend period: %w", err)
	}
	if period.CappedAt == nil {
		return nil
	}
	if agent.MonthlySpendCapMicros != nil && period.CostMicros >= *agent.MonthlySpendCapMicros {
		return nil
	}

	if err := models.New(tx).SetSpendCap(ctx, models.SetSpendCapParams{
		WorkspaceID: agent.WorkspaceID, PeriodStart: start, CappedAt: nil, Now: a.Clock.Now(),
	}); err != nil {
		return fmt.Errorf("lift spend cap: %w", err)
	}

	return nil
}

/*
applyPatch folds every field the caller sent into the agent row.
*/
func applyPatch(agent *models.AiAgent, p AgentPatch) {
	set(&agent.Name, p.Name)
	set(&agent.Tone, p.Tone)
	set(&agent.Guidance, p.Guidance)
	set(&agent.Schedule, p.Schedule)
	set(&agent.AssignedBehavior, p.AssignedBehavior)
	set(&agent.UnknownBehavior, p.UnknownBehavior)
	set(&agent.DefaultLanguage, p.DefaultLanguage)
	set(&agent.IncludeContactContext, p.IncludeContact)
	set(&agent.Enabled, p.Enabled)
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
	setOptional(&agent.MonthlySpendCapMicros, capMicros(p.MonthlyCapMicros))
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
capMicros maps a cap of zero or below to no cap at all: a cleared cap
field never pauses the agent.
*/
func capMicros(c Optional[int64]) Optional[int64] {
	if c.Set && c.Value != nil && *c.Value <= 0 {
		return Optional[int64]{Set: true}
	}
	return c
}

/*
checkPatch refuses a save that names rows outside the workspace or
switches on an agent that could not answer.
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

	if p.Enabled == nil || !*p.Enabled {
		return nil
	}

	return checkReady(ctx, tx, agent)
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
checkReady refuses to switch on an agent that could not answer,
naming every reason.
*/
func checkReady(ctx context.Context, tx *database.Tx, agent models.AiAgent) error {
	keys, err := models.New(tx).ListProviderKeys(ctx, agent.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list provider keys: %w", err)
	}

	ready, reasons := readiness(agent, keys)
	if ready {
		return nil
	}

	return fault.New(fault.Invalid, "agent_not_ready", "the agent cannot answer yet").
		OnField("enabled").With("reasons", reasons)
}

/*
updateParams maps the agent row to the update params.
*/
func updateParams(agent *models.AiAgent, now time.Time) models.UpdateAgentParams {
	return models.UpdateAgentParams{
		ID: agent.ID, WorkspaceID: agent.WorkspaceID,
		Name: agent.Name, AvatarAttachmentID: agent.AvatarAttachmentID, Enabled: agent.Enabled,
		ProductName: agent.ProductName, Tone: agent.Tone, Guidance: agent.Guidance,
		EnabledChannels: agent.EnabledChannels, ExcludedChannelConnectionIds: agent.ExcludedChannelConnectionIds,
		Schedule: agent.Schedule, OfficeHours: agent.OfficeHours, AssignedBehavior: agent.AssignedBehavior,
		AnswerKeyID: agent.AnswerKeyID, AnswerModel: agent.AnswerModel,
		ClassifierKeyID: agent.ClassifierKeyID, ClassifierModel: agent.ClassifierModel,
		FallbackKeyID: agent.FallbackKeyID, FallbackModel: agent.FallbackModel,
		Temperature: agent.Temperature, MaxOutputTokens: agent.MaxOutputTokens, MaxAiReplies: agent.MaxAiReplies,
		UnknownBehavior: agent.UnknownBehavior, GroundingMode: agent.GroundingMode,
		RequireCitations: agent.RequireCitations, MinSimilarity: agent.MinSimilarity,
		Languages: agent.Languages, DefaultLanguage: agent.DefaultLanguage, Streaming: agent.Streaming,
		IncludeContactContext:     agent.IncludeContactContext,
		ContactAttributeAllowlist: agent.ContactAttributeAllowlist,
		PiiRedaction:              agent.PiiRedaction, LinkAllowlist: agent.LinkAllowlist,
		Handoff: agent.Handoff, Messages: agent.Messages, PromptVersion: agent.PromptVersion,
		MonthlySpendCapMicros: agent.MonthlySpendCapMicros, ActionsEnabled: agent.ActionsEnabled,
		Now: now,
	}
}
