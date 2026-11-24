/*
|--------------------------------------------------------------------------
| Workspace AI Settings
|--------------------------------------------------------------------------
|
| What every agent and Operator share: the answer model an agent uses
| until it picks its own, the fallback when a provider fails, the model
| that classifies conversations and the monthly spend cap. Changes apply
| at once; they are the workspace's, not an agent's draft. Raising the
| cap above the month's spend lifts a pause the old cap caused.
|
*/

package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SettingsPatch is what one save of the workspace's AI settings changes.
Each field may be left out, set or cleared. A cap of zero or below, or
a cleared one, means no cap.
*/
type SettingsPatch struct {
	AnswerKeyID, ClassifierKeyID, FallbackKeyID Optional[ids.UUID]
	AnswerModel, ClassifierModel, FallbackModel Optional[string]
	MonthlyCapMicros                            Optional[int64]
}

/*
Settings returns the workspace's AI settings.
*/
func (a *Agents) Settings(ctx context.Context, m authz.Membership) (models.WorkspaceAiSetting, error) {
	if err := RequireAgentRead(m); err != nil {
		return models.WorkspaceAiSetting{}, err
	}
	var out models.WorkspaceAiSetting
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if _, _, err := a.Provision(ctx, tx, m.Workspace.ID); err != nil {
			return err
		}
		var err error
		out, err = agents.Shared(ctx, models.New(tx), m.Workspace.ID)
		return err
	})
	return out, err
}

/*
UpdateSettings saves the workspace's AI settings.

Errors: [exceptions.ErrForbidden], a [fault] naming a connection the
workspace does not have, or a wrapped database error.
*/
func (a *Agents) UpdateSettings(ctx context.Context, m authz.Membership, p SettingsPatch,
) (models.WorkspaceAiSetting, error) {
	if err := RequireAgentManage(m); err != nil {
		return models.WorkspaceAiSetting{}, err
	}
	var out models.WorkspaceAiSetting
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if _, _, err := a.Provision(ctx, tx, m.Workspace.ID); err != nil {
			return err
		}
		q := models.New(tx)
		s, err := agents.Shared(ctx, q, m.Workspace.ID)
		if err != nil {
			return err
		}
		for field, id := range map[string]Optional[ids.UUID]{
			"answer_key_id": p.AnswerKeyID, "classifier_key_id": p.ClassifierKeyID, "fallback_key_id": p.FallbackKeyID,
		} {
			if err := checkKey(ctx, tx, m.Workspace.ID, id); err != nil {
				return fault.New(fault.Invalid, "provider_key_not_found", "the connection does not exist").
					OnField(field)
			}
		}
		setOptional(&s.AnswerKeyID, p.AnswerKeyID)
		setOptional(&s.ClassifierKeyID, p.ClassifierKeyID)
		setOptional(&s.FallbackKeyID, p.FallbackKeyID)
		setText(&s.AnswerModel, p.AnswerModel)
		setText(&s.ClassifierModel, p.ClassifierModel)
		setText(&s.FallbackModel, p.FallbackModel)
		setOptional(&s.MonthlySpendCapMicros, capMicros(p.MonthlyCapMicros))

		out, err = q.UpdateWorkspaceAISettings(ctx, settingsParams(s, a.Clock))
		if err != nil {
			return fmt.Errorf("update workspace ai settings: %w", err)
		}
		if !p.MonthlyCapMicros.Set {
			return nil
		}
		return liftCap(ctx, q, a.Clock.Now(), out)
	})
	return out, err
}

/*
settingsParams maps the settings row to its update parameters.
*/
func settingsParams(s models.WorkspaceAiSetting, c clock.Clock) models.UpdateWorkspaceAISettingsParams {
	return models.UpdateWorkspaceAISettingsParams{
		WorkspaceID: s.WorkspaceID, AnswerKeyID: s.AnswerKeyID, AnswerModel: s.AnswerModel,
		ClassifierKeyID: s.ClassifierKeyID, ClassifierModel: s.ClassifierModel,
		FallbackKeyID: s.FallbackKeyID, FallbackModel: s.FallbackModel,
		MonthlySpendCapMicros: s.MonthlySpendCapMicros, Now: c.Now(),
	}
}

/*
capMicros maps a cap of zero or below to no cap at all: a cleared cap
field never pauses the agents.
*/
func capMicros(c Optional[int64]) Optional[int64] {
	if c.Set && c.Value != nil && *c.Value <= 0 {
		return Optional[int64]{Set: true}
	}
	return c
}

/*
liftCap clears the month's pause when the spend so far sits below the
new cap, or there is no cap any more.
*/
func liftCap(ctx context.Context, q *models.Queries, at time.Time, s models.WorkspaceAiSetting) error {
	now := at.UTC()
	start := database.Date(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC))

	period, err := q.GetSpendPeriod(ctx, models.GetSpendPeriodParams{WorkspaceID: s.WorkspaceID, PeriodStart: start})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find spend period: %w", err)
	}
	if period.CappedAt == nil {
		return nil
	}
	if s.MonthlySpendCapMicros != nil && period.CostMicros >= *s.MonthlySpendCapMicros {
		return nil
	}
	if err := q.SetSpendCap(ctx, models.SetSpendCapParams{
		WorkspaceID: s.WorkspaceID, PeriodStart: start, CappedAt: nil, Now: at,
	}); err != nil {
		return fmt.Errorf("lift spend cap: %w", err)
	}
	return nil
}
