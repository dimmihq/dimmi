/*
|--------------------------------------------------------------------------
| Operator Settings
|--------------------------------------------------------------------------
|
| Every teammate may read how Operator is set up, since the inbox needs
| to know whether to brief a conversation on its own. Only those who
| manage the AI agent change it, and a change merges into what is saved:
| fields left out keep their values, and a policy left out keeps its own.
|
*/

package operatorchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/ai/operator"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxInstructions is the longest the workspace's instructions may be.
*/
const maxInstructions = 8000

/*
SettingsInput is a change to Operator's settings; nil fields stay.
*/
type SettingsInput struct {
	Membership      authz.Membership
	Enabled         *bool
	BriefOnOpen     *bool
	TriageOnArrival *bool
	Instructions    *string
	Policies        map[string]string
	Model           *string
}

/*
Settings reads and changes Operator's settings.

Rules:
  - Reading needs conversations:read; changing needs ai_agent:manage.
  - Policies are auto, ask or off for a known group; articles and
    procedures can never be auto.
  - Instructions are at most 8000 characters.
  - A model is one the agent's connection offers; empty uses the
    agent's own model.

Errors: [exceptions.ErrForbidden], a [fault] for an invalid value, or a
wrapped database error.
*/
type Settings struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Show returns the workspace's settings.
*/
func (a *Settings) Show(ctx context.Context, m authz.Membership) (operator.Settings, error) {
	if err := conversations.CanRead(m); err != nil {
		return operator.Settings{}, err
	}
	var out operator.Settings
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = operator.LoadSettings(ctx, models.New(tx), m.Workspace.ID)
		return err
	})
	return out, err
}

/*
Update merges the change into the saved settings.
*/
func (a *Settings) Update(ctx context.Context, in SettingsInput) (operator.Settings, error) {
	if !in.Membership.Can("ai_agent:manage") {
		return operator.Settings{}, exceptions.ErrForbidden
	}
	if err := check(in); err != nil {
		return operator.Settings{}, err
	}
	var out operator.Settings
	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := operator.LoadSettings(ctx, q, in.Membership.Workspace.ID)
		if err != nil {
			return err
		}
		if err := checkModel(ctx, q, in); err != nil {
			return err
		}
		out = merge(current, in)
		policies, _ := json.Marshal(out.Policies)
		_, err = q.SaveOperatorSettings(ctx, models.SaveOperatorSettingsParams{
			WorkspaceID: in.Membership.Workspace.ID, Enabled: out.Enabled, BriefOnOpen: out.BriefOnOpen,
			TriageOnArrival: out.TriageOnArrival, Instructions: out.Instructions, Policies: policies,
			Model: optional(out.Model), Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("save operator settings: %w", err)
		}
		return nil
	})
	return out, err
}

/*
check refuses unknown groups, unknown policies and instructions that
are too long.
*/
func check(in SettingsInput) error {
	if in.Instructions != nil && len([]rune(*in.Instructions)) > maxInstructions {
		return fault.New(fault.Invalid, "instructions_too_long", "Instructions are at most 8000 characters.").
			OnField("instructions")
	}
	for group, policy := range in.Policies {
		if !operator.ValidGroup(group) || !operator.ValidPolicy(operator.Policy(policy)) {
			return fault.New(fault.Invalid, "policy_invalid", "Each policy is auto, ask or off for a known group.").
				OnField("policies")
		}
		if operator.Policy(policy) == operator.PolicyAuto && !operator.CanAuto(group) {
			return fault.New(fault.Invalid, "policy_not_auto",
				"Article and procedure changes are always reviewed, so they cannot apply on their own.").
				OnField("policies")
		}
	}
	return nil
}

/*
merge writes a change over the saved settings.
*/
func merge(s operator.Settings, in SettingsInput) operator.Settings {
	if in.Enabled != nil {
		s.Enabled = *in.Enabled
	}
	if in.BriefOnOpen != nil {
		s.BriefOnOpen = *in.BriefOnOpen
	}
	if in.TriageOnArrival != nil {
		s.TriageOnArrival = *in.TriageOnArrival
	}
	if in.Instructions != nil {
		s.Instructions = *in.Instructions
	}
	if in.Model != nil {
		s.Model = strings.TrimSpace(*in.Model)
	}
	policies := maps.Clone(s.Policies)
	if policies == nil {
		policies = map[operator.Group]operator.Policy{}
	}
	for group, policy := range in.Policies {
		policies[group] = operator.Policy(policy)
	}
	s.Policies = policies
	return s
}

/*
checkModel refuses a model the agent's connection does not offer.
*/
func checkModel(ctx context.Context, q *models.Queries, in SettingsInput) error {
	if in.Model == nil || strings.TrimSpace(*in.Model) == "" {
		return nil
	}
	refused := fault.New(fault.Invalid, "model_unavailable", "The agent's AI connection does not offer this model.").
		OnField("model")
	agent, err := q.FindAgent(ctx, in.Membership.Workspace.ID)
	if errors.Is(err, database.ErrNotFound) || (err == nil && agent.AnswerKeyID == nil) {
		return refused
	}
	if err != nil {
		return fmt.Errorf("find agent: %w", err)
	}
	key, err := q.FindProviderKey(ctx, models.FindProviderKeyParams{
		WorkspaceID: agent.WorkspaceID, ID: *agent.AnswerKeyID,
	})
	if err != nil || !operator.Offers(key, strings.TrimSpace(*in.Model)) {
		return refused
	}
	return nil
}

/*
optional is nil for an empty text.
*/
func optional(text string) *string {
	if text == "" {
		return nil
	}
	return &text
}
