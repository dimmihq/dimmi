/*
|--------------------------------------------------------------------------
| Update Redaction Rule
|--------------------------------------------------------------------------
|
| A rule is renamed, given a new pattern or replacement, or switched off
| in place. Messages it already changed stay as they were stored.
|
*/

package redaction

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateRuleInput is who changes which rule and how.
*/
type UpdateRuleInput struct {
	Membership authz.Membership
	RuleID     ids.UUID
	Fields     RuleFields
}

/*
UpdateRule changes a rule.

Rules:
  - The caller needs settings:write and redaction:manage.
  - A new pattern compiles. Values equal to the current ones change
    nothing.

Events: [appevents.RedactionRuleUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrInvalidRedactionPattern], or a wrapped database error.
*/
type UpdateRule struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle changes the rule and returns it as it now is.
*/
func (a *UpdateRule) Handle(ctx context.Context, in UpdateRuleInput) (models.RedactionRule, error) {
	if !canManage(in.Membership) {
		return models.RedactionRule{}, exceptions.ErrForbidden
	}

	var out models.RedactionRule

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		current, err := find(ctx, q, ws, in.RuleID)
		if err != nil {
			return err
		}

		v := in.Fields.over(ruleValues{
			name: current.Name, pattern: current.Pattern, replacement: current.Replacement, enabled: current.Enabled,
		})
		params := models.UpdateRedactionRuleParams{
			Name: v.name, Pattern: v.pattern, Replacement: v.replacement, Enabled: v.enabled,
			WorkspaceID: ws, ID: current.ID, Now: a.Clock.Now(),
		}

		out = current

		changes := changed(current, params)
		if len(changes) == 0 {
			return nil
		}

		if err := checkPattern(params.Pattern); err != nil {
			return err
		}

		if out, err = q.UpdateRedactionRule(ctx, params); err != nil {
			return fmt.Errorf("update redaction rule: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.RedactionRuleUpdated{
			Base: base(in.Membership, params.Now), RuleID: out.ID, Name: out.Name, Changes: changes,
		})
	})

	return out, err
}

/*
changed lists the fields the update changes.
*/
func changed(current models.RedactionRule, next models.UpdateRedactionRuleParams) []string {
	var out []string

	for _, field := range []struct {
		name string
		same bool
	}{
		{"name", current.Name == next.Name},
		{"pattern", current.Pattern == next.Pattern},
		{"replacement", current.Replacement == next.Replacement},
		{"enabled", current.Enabled == next.Enabled},
	} {
		if !field.same {
			out = append(out, field.name)
		}
	}

	return out
}
