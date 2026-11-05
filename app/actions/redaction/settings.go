/*
|--------------------------------------------------------------------------
| Redaction Settings
|--------------------------------------------------------------------------
|
| The built-in rules are switched on and off, and the workspace's own
| rules put in a new order, with one request on the rule collection. An
| order names every rule exactly once, so a screen that missed a rule
| another teammate just added is refused rather than half applied.
|
*/

package redaction

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/redaction"
)

/*
Rules is a workspace's redaction: the built-in switches and its own
rules in order.
*/
type Rules struct {
	Settings redaction.Settings
	Rules    []models.RedactionRule
}

/*
ListRulesInput is who reads the rules.
*/
type ListRulesInput struct {
	Membership authz.Membership
}

/*
ListRules returns the workspace's redaction.

Rules:
  - The caller needs settings:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListRules struct {
	DB *database.DB
}

/*
Handle returns the switches and the rules.
*/
func (a *ListRules) Handle(ctx context.Context, in ListRulesInput) (Rules, error) {
	if !in.Membership.Can("settings:read") {
		return Rules{}, exceptions.ErrForbidden
	}

	var out Rules

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = current(ctx, models.New(tx), in.Membership.Workspace.ID)

		return err
	})

	return out, err
}

/*
current reads the workspace's switches and rules.
*/
func current(ctx context.Context, q *models.Queries, ws ids.UUID) (Rules, error) {
	settings, err := readSettings(ctx, q, ws)
	if err != nil {
		return Rules{}, err
	}

	rules, err := q.ListRedactionRules(ctx, ws)
	if err != nil {
		return Rules{}, fmt.Errorf("list redaction rules: %w", err)
	}

	return Rules{Settings: settings, Rules: rules}, nil
}

/*
UpdateSettingsInput is who switches which built-in rules, and the new
order of the workspace's rules, nil to keep it.
*/
type UpdateSettingsInput struct {
	Membership  authz.Membership
	CardNumbers *bool
	USSSN       *bool
	Order       []ids.UUID
}

/*
UpdateSettings switches the built-in rules and reorders the others.

Rules:
  - The caller needs settings:write and redaction:manage.
  - An order lists each of the workspace's rules exactly once.

Events: [appevents.RedactionSettingsUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrRedactionOrder], or a
wrapped database error.
*/
type UpdateSettings struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle applies the change and returns the redaction as it now is.
*/
func (a *UpdateSettings) Handle(ctx context.Context, in UpdateSettingsInput) (Rules, error) {
	if !canManage(in.Membership) {
		return Rules{}, exceptions.ErrForbidden
	}

	var out Rules

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), in.Membership.Workspace.ID, a.Clock.Now()

		before, err := current(ctx, q, ws)
		if err != nil {
			return err
		}

		changes, err := a.switchBuiltIns(ctx, q, before.Settings, in)
		if err != nil {
			return err
		}

		moved, err := a.reorder(ctx, q, before.Rules, in)
		if err != nil {
			return err
		}

		if moved {
			changes["order"] = len(in.Order)
		}

		if out, err = current(ctx, q, ws); err != nil || len(changes) == 0 {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.RedactionSettingsUpdated{
			Base: base(in.Membership, now), Changes: changes,
		})
	})

	return out, err
}

/*
switchBuiltIns saves the switches that changed and returns them as
changes.
*/
func (a *UpdateSettings) switchBuiltIns(
	ctx context.Context, q *models.Queries, before redaction.Settings, in UpdateSettingsInput,
) (map[string]any, error) {
	next, changes := before, map[string]any{}

	for _, s := range []struct {
		key    string
		value  *bool
		target *bool
	}{
		{redaction.RuleCardNumbers, in.CardNumbers, &next.CardNumbers},
		{redaction.RuleUSSSN, in.USSSN, &next.USSSN},
	} {
		if s.value != nil && *s.value != *s.target {
			changes[s.key] = map[string]any{"from": *s.target, "to": *s.value}
			*s.target = *s.value
		}
	}

	if len(changes) == 0 {
		return changes, nil
	}

	encoded, err := json.Marshal(next)
	if err != nil {
		return nil, fmt.Errorf("encode redaction settings: %w", err)
	}

	err = q.SetRedactionSettings(ctx, models.SetRedactionSettingsParams{
		Redaction: encoded, Now: a.Clock.Now(), WorkspaceID: in.Membership.Workspace.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("save redaction settings: %w", err)
	}

	return changes, nil
}

/*
reorder gives the rules the positions of the order and reports whether
any rule moved.
*/
func (a *UpdateSettings) reorder(
	ctx context.Context, q *models.Queries, rules []models.RedactionRule, in UpdateSettingsInput,
) (bool, error) {
	if in.Order == nil {
		return false, nil
	}

	if !sameRules(rules, in.Order) {
		return false, exceptions.ErrRedactionOrder
	}

	moved := false

	for position, id := range in.Order {
		moved = moved || rules[position].ID != id

		err := q.MoveRedactionRule(ctx, models.MoveRedactionRuleParams{
			Position: int32(position), WorkspaceID: in.Membership.Workspace.ID, ID: id, Now: a.Clock.Now(),
		})
		if err != nil {
			return false, fmt.Errorf("move redaction rule: %w", err)
		}
	}

	return moved, nil
}

/*
sameRules reports whether the order names each rule exactly once.
*/
func sameRules(rules []models.RedactionRule, order []ids.UUID) bool {
	if len(rules) != len(order) {
		return false
	}

	for _, rule := range rules {
		if !slices.Contains(order, rule.ID) {
			return false
		}
	}

	return true
}
