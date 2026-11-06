/*
|--------------------------------------------------------------------------
| Eligibility Gate
|--------------------------------------------------------------------------
|
| Before any model is called, the gate answers whether the agent may
| speak here at all: the agent is on, the channel is allowed, the
| conversation is live and handed to no one, the writer is a real
| customer, and the budgets hold. A "no" is a skip, never an error.
|
*/

package pipeline

import (
	"context"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
	"github.com/jackc/pgx/v5/pgtype"
)

/*
skip names why the agent stayed quiet.
*/
type skip struct {
	Reason string
}

/*
gate runs the eligibility checks in order and returns the first
reason the agent may not answer, or nil.
*/
func (r *Runner) gate(ctx context.Context, env *env) *skip {
	if env.Agent == nil {
		return &skip{Reason: "agent_missing"}
	}
	if !env.Agent.Enabled {
		return &skip{Reason: "agent_disabled"}
	}
	if len(env.Agent.EnabledChannels) > 0 && !slices.Contains(env.Agent.EnabledChannels, env.ChannelType) {
		return &skip{Reason: "channel_excluded"}
	}
	if slices.Contains(env.Agent.ExcludedChannelConnectionIds, env.Conversation.ChannelConnectionID) {
		return &skip{Reason: "connection_excluded"}
	}
	if env.Conversation.AiState != "active" && env.Conversation.AiState != "off" {
		return &skip{Reason: "ai_handed_off"}
	}
	if env.Conversation.DeletedAt != nil {
		return &skip{Reason: "conversation_deleted"}
	}
	if env.Conversation.Spam {
		return &skip{Reason: "conversation_spam"}
	}
	if env.Conversation.Status == "closed" {
		return &skip{Reason: "conversation_closed"}
	}
	if env.Conversation.AiReplyCount >= env.Agent.MaxAiReplies {
		return &skip{Reason: "max_ai_replies"}
	}
	if env.Agent.AssignedBehavior == "never_when_assigned" &&
		(env.Conversation.AssigneeMemberID != nil || env.Conversation.AssigneeTeamID != nil) {
		return &skip{Reason: "assigned"}
	}
	if reason := r.scheduleGate(env); reason != "" {
		return &skip{Reason: reason}
	}
	if reason := r.spendGate(ctx, env); reason != "" {
		return &skip{Reason: reason}
	}
	if reason := r.readinessGate(env); reason != "" {
		return &skip{Reason: reason}
	}
	return nil
}

/*
scheduleGate applies the schedule setting.
*/
func (r *Runner) scheduleGate(env *env) string {
	if env.Agent.Schedule == "always" {
		return ""
	}
	if env.Agent.OfficeHours == nil {
		return ""
	}
	within := officehours.Within(env.Agent.OfficeHours, r.Clock.Now())
	if env.Agent.Schedule == "outside_office_hours" && within {
		return "schedule_inside_hours"
	}
	if env.Agent.Schedule == "inside_office_hours" && !within {
		return "schedule_outside_hours"
	}
	return ""
}

/*
spendGate pauses when the workspace is over its monthly cap.
*/
func (r *Runner) spendGate(ctx context.Context, env *env) string {
	if env.Agent.MonthlySpendCapMicros == nil {
		return ""
	}
	now := r.Clock.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	periodRow, err := models.New(env.tx).GetSpendPeriod(ctx, models.GetSpendPeriodParams{
		WorkspaceID: env.Workspace.ID, PeriodStart: pgtype.Date{Time: start, Valid: true},
	})
	if err != nil {
		return ""
	}
	if periodRow.CappedAt != nil {
		return "spend_cap"
	}
	if periodRow.CostMicros >= *env.Agent.MonthlySpendCapMicros {
		return "spend_cap"
	}
	return ""
}

/*
readinessGate needs an answer model and a usable key.
*/
func (r *Runner) readinessGate(env *env) string {
	if env.Agent.AnswerModel == nil || env.Agent.AnswerKeyID == nil {
		return "no_answer_model"
	}
	return ""
}

