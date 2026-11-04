/*
|--------------------------------------------------------------------------
| Access Policy
|--------------------------------------------------------------------------
|
| A workspace may ask more of the people working in it than a valid
| credential: today a second factor on every member's account. Once the
| caller and the workspace are known, the access policy checks each rule
| the workspace sets, for browsers, apps and access tokens alike, so a
| member without a second factor cannot reach the workspace through a
| token they made before it was required either.
|
| Every refusal has the same shape: what is required and the page where
| the person meets the requirement, so clients handle a new rule without
| a change. The check reads only what the request already loaded, the
| caller's account and the workspace, and costs no query, so a change to
| either applies to the very next request.
|
| Routes that let a person meet a requirement, or walk away from it, sit
| outside the workspace and never run the policy: the account's own
| pages, its factors, its sessions and leaving a workspace.
|
*/

package auth

import (
	"context"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
RequirementTwoFactor names the requirement of a second factor in a
refusal.
*/
const RequirementTwoFactor = "two_factor"

/*
AccessPolicy decides whether a caller may work in a workspace beyond
holding a valid credential and an active membership.
*/
type AccessPolicy interface {
	/*
		Evaluate returns nil when the caller may work in the workspace,
		or the refusal of the first requirement they do not meet.
	*/
	Evaluate(ctx context.Context, p *Principal, w models.Workspace) error
}

/*
AccessRule is one requirement a workspace may set.
*/
type AccessRule interface {
	/*
		Check returns nil when the caller meets the requirement in the
		workspace, or its refusal.
	*/
	Check(p *Principal, w models.Workspace) error
}

/*
AccessRules is the access policy that checks each rule in turn.
*/
type AccessRules []AccessRule

var _ AccessPolicy = AccessRules(nil)

/*
Evaluate returns the refusal of the first rule the caller fails.
*/
func (r AccessRules) Evaluate(_ context.Context, p *Principal, w models.Workspace) error {
	for _, rule := range r {
		if err := rule.Check(p, w); err != nil {
			return err
		}
	}

	return nil
}

/*
RequireTwoFactor is the rule of a workspace that requires a second
factor on every member's account. ActionURL is the page where the
person sets one up.
*/
type RequireTwoFactor struct {
	ActionURL string
}

/*
Check refuses a caller without a second factor in a workspace that
requires one.
*/
func (r RequireTwoFactor) Check(p *Principal, w models.Workspace) error {
	if !w.RequiresTwoFactor() || p.User.TwoFactorEnabled() {
		return nil
	}

	return exceptions.ErrTwoFactorRequired.
		With("requirement", RequirementTwoFactor).
		With("action_url", r.ActionURL)
}

/*
DefaultAccessPolicy returns the rules every workspace may set.
*/
func DefaultAccessPolicy() AccessRules {
	return AccessRules{RequireTwoFactor{ActionURL: SecurityPage}}
}
