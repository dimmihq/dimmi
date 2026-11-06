/*
|--------------------------------------------------------------------------
| Update AI Agent
|--------------------------------------------------------------------------
|
| The agent's whole settings surface in one save: identity, models,
| rules, channels, schedule, handoff, privacy and spend. Whatever the
| caller leaves unset keeps its stored value.
|
*/

package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/config"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/fault"
)

/*
UpdateAgentInput is the full mutable agent shape; nil fields keep
their stored values.
*/
type UpdateAgentInput struct {
	Membership authz.Membership
	Agent      *models.AiAgent
}

/*
UpdateAiAgent saves the agent's settings.

Rules:
  - The caller needs ai:write.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
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
	if err := RequireAgentWrite(in.Membership); err != nil {
		return nil, err
	}
	if in.Agent == nil {
		return nil, fault.New(fault.Invalid, "agent_required", "the agent is required")
	}

	var out models.AiAgent

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		existing, err := ensureAgent(ctx, a.Clock, a.Config, tx, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}

		in.Agent.ID = existing.ID

		out, err = models.New(tx).UpdateAgent(ctx, updateParams(in.Agent, a.Clock.Now()))
		if err != nil {
			return fmt.Errorf("update ai agent: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &out, nil
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
