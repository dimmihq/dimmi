/*
|--------------------------------------------------------------------------
| Redaction Controllers
|--------------------------------------------------------------------------
|
| GET /redaction-rules returns the built-in switches and the workspace's
| own rules in order, and PATCH /redaction-rules switches the built-ins
| and reorders the rules. POST, PATCH and DELETE on the rules add, change
| and remove one. POST /redaction-tests shows what the rules make of a
| text without storing it.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/redaction"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RedactionRulesOutput is the workspace's redaction.
*/
type RedactionRulesOutput struct {
	Body fhttp.Resource[resources.RedactionRules]
}

/*
RedactionRuleOutput is one rule.
*/
type RedactionRuleOutput struct {
	Body fhttp.Resource[resources.RedactionRule]
}

/*
RedactionTestOutput is a tried text.
*/
type RedactionTestOutput struct {
	Body fhttp.Resource[resources.RedactionTest]
}

/*
RedactionController serves the workspace's redaction rules.
*/
type RedactionController struct {
	List     *redaction.ListRules
	Settings *redaction.UpdateSettings
	Create   *redaction.CreateRule
	Change   *redaction.UpdateRule
	Remove   *redaction.DeleteRule
	Try      *redaction.TryRedaction
	IDs      *ids.Registry
}

/*
Index returns the built-in switches and the rules.
*/
func (c *RedactionController) Index(ctx context.Context, _ *requests.NoInput) (*RedactionRulesOutput, error) {
	rules, err := c.List.Handle(ctx, redaction.ListRulesInput{Membership: membership(ctx)})
	if err != nil {
		return nil, err
	}

	return &RedactionRulesOutput{Body: fhttp.NewResource(resources.NewRedactionRules(rules))}, nil
}

/*
UpdateSettings switches the built-ins, reorders the rules, and answers
with the redaction as it now is.
*/
func (c *RedactionController) UpdateSettings(
	ctx context.Context, in *requests.UpdateRedactionSettingsRequest,
) (*RedactionRulesOutput, error) {
	order, err := c.order(in.Body.Order)
	if err != nil {
		return nil, err
	}

	rules, err := c.Settings.Handle(ctx, redaction.UpdateSettingsInput{
		Membership: membership(ctx), CardNumbers: in.Body.CardNumbers, USSSN: in.Body.USSSN, Order: order,
	})
	if err != nil {
		return nil, err
	}

	return &RedactionRulesOutput{Body: fhttp.NewResource(resources.NewRedactionRules(rules))}, nil
}

/*
order decodes the rule ids of an order, nil when none was sent.
*/
func (c *RedactionController) order(public []string) ([]ids.UUID, error) {
	if public == nil {
		return nil, nil
	}

	out := make([]ids.UUID, 0, len(public))

	for _, id := range public {
		decoded, err := c.IDs.Decode(string(models.PrefixRedactionRule), id)
		if err != nil {
			return nil, exceptions.ErrRedactionOrder
		}

		out = append(out, decoded)
	}

	return out, nil
}

/*
Store adds a rule and answers with it.
*/
func (c *RedactionController) Store(
	ctx context.Context, in *requests.StoreRedactionRuleRequest,
) (*RedactionRuleOutput, error) {
	rule, err := c.Create.Handle(ctx, redaction.CreateRuleInput{
		Membership: membership(ctx),
		Fields: redaction.RuleFields{
			Name: &in.Body.Name, Pattern: &in.Body.Pattern, Replacement: in.Body.Replacement, Enabled: in.Body.Enabled,
		},
	})
	if err != nil {
		return nil, err
	}

	return &RedactionRuleOutput{Body: fhttp.NewResource(resources.NewRedactionRule(rule))}, nil
}

/*
Update changes a rule and answers with it as it now is.
*/
func (c *RedactionController) Update(
	ctx context.Context, in *requests.UpdateRedactionRuleRequest,
) (*RedactionRuleOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixRedactionRule), in.Rule)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	rule, err := c.Change.Handle(ctx, redaction.UpdateRuleInput{
		Membership: membership(ctx), RuleID: id,
		Fields: redaction.RuleFields{
			Name: in.Body.Name, Pattern: in.Body.Pattern, Replacement: in.Body.Replacement, Enabled: in.Body.Enabled,
		},
	})
	if err != nil {
		return nil, err
	}

	return &RedactionRuleOutput{Body: fhttp.NewResource(resources.NewRedactionRule(rule))}, nil
}

/*
Destroy removes a rule.
*/
func (c *RedactionController) Destroy(ctx context.Context, in *requests.RedactionRuleRequest) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixRedactionRule), in.Rule)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	if err := c.Remove.Handle(ctx, redaction.DeleteRuleInput{Membership: membership(ctx), RuleID: id}); err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}

/*
Test answers with what the rules make of the text.
*/
func (c *RedactionController) Test(
	ctx context.Context, in *requests.TryRedactionRequest,
) (*RedactionTestOutput, error) {
	tried, err := c.Try.Handle(ctx, redaction.TryRedactionInput{Membership: membership(ctx), Text: in.Body.Text})
	if err != nil {
		return nil, err
	}

	return &RedactionTestOutput{Body: fhttp.NewResource(resources.NewRedactionTest(tried))}, nil
}
