/*
|--------------------------------------------------------------------------
| Access Policy
|--------------------------------------------------------------------------
|
| A workspace may ask more of the people working in it than a valid
| credential. Once the caller and the workspace are known, the access
| policy checks each rule the workspace sets, for browsers, apps and
| access tokens alike, so a member without a second factor cannot reach
| the workspace through a token they made before it was required either.
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
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/enums"
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
		Evaluate returns nil when the caller, holding the role in the
		workspace, may work in it, or the refusal of the first requirement
		they do not meet.
	*/
	Evaluate(ctx context.Context, p *Principal, w models.Workspace, role enums.Role) error
}

/*
AccessCheck is one request being weighed against a workspace's rules:
the caller, the role they hold there, the rules decoded once for the
whole request, and the time.
*/
type AccessCheck struct {
	Principal *Principal
	Role      enums.Role
	Settings  models.SecuritySettings
	Now       time.Time
}

/*
AccessRule is one requirement a workspace may set.
*/
type AccessRule interface {
	/*
		Check returns nil when the caller meets the requirement, or its
		refusal.
	*/
	Check(a AccessCheck) error
}

/*
AccessRules is the access policy that checks each rule in turn, reading
the time from Clock.
*/
type AccessRules struct {
	Clock clock.Clock
	Rules []AccessRule
}

var _ AccessPolicy = AccessRules{}

/*
Evaluate returns the refusal of the first rule the caller fails.
*/
func (r AccessRules) Evaluate(_ context.Context, p *Principal, w models.Workspace, role enums.Role) error {
	check := AccessCheck{Principal: p, Role: role, Settings: w.Security(), Now: r.Clock.Now()}

	for _, rule := range r.Rules {
		if err := rule.Check(check); err != nil {
			return err
		}
	}

	return nil
}

/*
RequireTwoFactor is the rule of a workspace that requires a second
factor on every member's account. ActionURL is the page where the
person sets one up. The company's identity provider answers for it on
a session through one of the organization's own connections, and nowhere
else.
*/
type RequireTwoFactor struct {
	ActionURL string
}

/*
Check refuses a caller without a second factor in a workspace that
requires one.
*/
func (r RequireTwoFactor) Check(a AccessCheck) error {
	if !a.Settings.RequireTwoFactor || a.Principal.User.TwoFactorEnabled() {
		return nil
	}

	connection := a.Principal.SSOConnectionID
	if connection != ids.Nil() && slices.Contains(a.Settings.SSOConnectionIDs, connection) {
		return nil
	}

	return exceptions.ErrTwoFactorRequired.
		With("requirement", RequirementTwoFactor).
		With("action_url", r.ActionURL)
}

/*
SSORedirectPath is where a sign-in through the connection starts.
*/
func SSORedirectPath(connection ids.UUID) string {
	return "/auth/sso/" + models.PublicID(models.PrefixSSOConnection, connection) + "/redirect"
}

/*
DefaultAccessPolicy returns the rules every workspace may set, in the
order a caller is asked to meet them.
*/
func DefaultAccessPolicy(clk clock.Clock) AccessRules {
	return AccessRules{Clock: clk, Rules: []AccessRule{
		AddressRule{},
		SSORule{Redirect: SSORedirectPath},
		SignInMethodRule{},
		GoogleDomainRule{},
		SessionLengthRule{},
		RequireTwoFactor{ActionURL: SecurityPage},
	}}
}
