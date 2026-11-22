/*
|--------------------------------------------------------------------------
| Workspace AI Settings
|--------------------------------------------------------------------------
|
| What every agent and Operator share: the default answer model, the
| classifier, the fallback and the monthly spend cap. An agent that sets
| no answer model of its own answers with the workspace's.
|
*/

package agents

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Defaults are what a workspace's AI settings start from: the monthly
cap in dollars, and when they are made.
*/
type Defaults struct {
	CapUSD float64
	Now    time.Time
}

/*
Settings returns the workspace's AI settings, making them from the
defaults when missing.
*/
func Settings(ctx context.Context, q *models.Queries, workspaceID ids.UUID, d Defaults,
) (models.WorkspaceAiSetting, error) {
	capMicros := int64(d.CapUSD * 1_000_000)
	now := d.Now
	s, err := q.EnsureWorkspaceAISettings(ctx, models.EnsureWorkspaceAISettingsParams{
		WorkspaceID: workspaceID, MonthlySpendCapMicros: &capMicros, Now: now,
	})
	if err != nil {
		return models.WorkspaceAiSetting{}, fmt.Errorf("ensure workspace ai settings: %w", err)
	}
	return models.WorkspaceAiSetting(s), nil
}

/*
Answering returns the agent as it answers: its own answer and fallback
models, or the workspace's where it names none, and the workspace's
classifier and spend cap, which every agent shares.
*/
func Answering(agent models.AiAgent, s models.WorkspaceAiSetting) models.AiAgent {
	if agent.AnswerKeyID == nil {
		agent.AnswerKeyID, agent.AnswerModel = s.AnswerKeyID, s.AnswerModel
	}
	if agent.FallbackKeyID == nil {
		agent.FallbackKeyID, agent.FallbackModel = s.FallbackKeyID, s.FallbackModel
	}
	agent.ClassifierKeyID, agent.ClassifierModel = s.ClassifierKeyID, s.ClassifierModel
	agent.MonthlySpendCapMicros = s.MonthlySpendCapMicros
	return agent
}

/*
DefaultCapMicros is the monthly cap a workspace without AI settings
yet is held to, as the settings table defaults it.
*/
const DefaultCapMicros int64 = 25_000_000

/*
Shared returns the workspace's AI settings, or the defaults when it has
none stored yet. It writes nothing, so answering a message never does.
*/
func Shared(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (models.WorkspaceAiSetting, error) {
	s, err := q.FindWorkspaceAISettings(ctx, workspaceID)
	if errors.Is(err, database.ErrNotFound) {
		capMicros := DefaultCapMicros
		return models.WorkspaceAiSetting{WorkspaceID: workspaceID, MonthlySpendCapMicros: &capMicros}, nil
	}
	if err != nil {
		return models.WorkspaceAiSetting{}, fmt.Errorf("find workspace ai settings: %w", err)
	}
	return s, nil
}

/*
Answerer resolves the agent that answers and returns it as it answers,
with the workspace's shared models and cap folded in.

Errors: [ErrNoAgent], or a wrapped database error.
*/
func Answerer(ctx context.Context, q *models.Queries, w Where) (models.AiAgent, error) {
	agent, err := Resolve(ctx, q, w)
	if err != nil {
		return agent, err
	}
	s, err := Shared(ctx, q, w.WorkspaceID)
	if err != nil {
		return agent, err
	}
	return Answering(agent, s), nil
}

/*
DefaultAnswerer returns the workspace's default agent as it answers.

Errors: [ErrNoAgent], or a wrapped database error.
*/
func DefaultAnswerer(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (models.AiAgent, error) {
	return Answerer(ctx, q, Where{WorkspaceID: workspaceID})
}
