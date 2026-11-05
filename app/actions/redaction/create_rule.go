/*
|--------------------------------------------------------------------------
| Create Redaction Rule
|--------------------------------------------------------------------------
|
| A rule is added at the end of the order. Its pattern is compiled before
| it is saved, so a broken one is refused with the request rather than
| failing a customer's message later.
|
*/

package redaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/redaction"
)

/*
RuleFields are a rule's values. A nil field keeps the current value on
an update and takes the default on create: the default replacement, and
enabled.
*/
type RuleFields struct {
	Name        *string
	Pattern     *string
	Replacement *string
	Enabled     *bool
}

/*
CreateRuleInput is who adds which rule.
*/
type CreateRuleInput struct {
	Membership authz.Membership
	Fields     RuleFields
}

/*
CreateRule adds a rule after the workspace's others.

Rules:
  - The caller needs settings:write and redaction:manage.
  - The pattern compiles; a workspace keeps at most MaxRules.

Events: [appevents.RedactionRuleCreated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrInvalidRedactionPattern],
[exceptions.ErrRedactionRuleLimitReached], or a wrapped database error.
*/
type CreateRule struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle adds the rule and returns it.
*/
func (a *CreateRule) Handle(ctx context.Context, in CreateRuleInput) (models.RedactionRule, error) {
	if !canManage(in.Membership) {
		return models.RedactionRule{}, exceptions.ErrForbidden
	}

	v := in.Fields.over(ruleValues{replacement: redaction.DefaultReplacement, enabled: true})
	params := models.CreateRedactionRuleParams{
		WorkspaceID: in.Membership.Workspace.ID, Name: v.name, Pattern: v.pattern, Replacement: v.replacement,
		Enabled: v.enabled,
	}

	if err := checkPattern(params.Pattern); err != nil {
		return models.RedactionRule{}, err
	}

	var out models.RedactionRule

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		count, err := q.CountRedactionRules(ctx, params.WorkspaceID)
		if err != nil {
			return fmt.Errorf("count redaction rules: %w", err)
		}

		if count >= MaxRules {
			return exceptions.ErrRedactionRuleLimitReached
		}

		params.Now = a.Clock.Now()

		if out, err = q.CreateRedactionRule(ctx, params); err != nil {
			return fmt.Errorf("create redaction rule: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.RedactionRuleCreated{
			Base: base(in.Membership, params.Now), RuleID: out.ID, Name: out.Name,
		})
	})

	return out, err
}

/*
ruleValues are a rule's values as they will be saved.
*/
type ruleValues struct {
	name, pattern, replacement string
	enabled                    bool
}

/*
over returns the values with the fields that were sent written over
them.
*/
func (f RuleFields) over(v ruleValues) ruleValues {
	if f.Name != nil {
		v.name = strings.TrimSpace(*f.Name)
	}

	if f.Pattern != nil {
		v.pattern = *f.Pattern
	}

	if f.Replacement != nil {
		v.replacement = *f.Replacement
	}

	if f.Enabled != nil {
		v.enabled = *f.Enabled
	}

	return v
}
