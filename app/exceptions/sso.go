/*
|--------------------------------------------------------------------------
| Single Sign-On Errors
|--------------------------------------------------------------------------
|
| What can go wrong while a company connects its identity provider and
| proves its domains. Each refusal says what to fix: a connection needs a
| verified domain and HTTPS before it signs anyone in, an enforced one
| cannot be removed from under the people it binds, and a domain anyone
| can register mail under is never claimable.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Refusals of connections.

ErrNoVerifiedDomain refuses to activate a connection no verified domain
routes to. ErrHTTPSRequired refuses one on an install served over plain
HTTP. ErrConnectionEnforced refuses to delete an enforced connection.
ErrConnectionNotActive refuses to enforce a connection that is not active.
ErrEnforceUnproven refuses to enforce a connection nobody has just signed
in through. ErrConnectionLimitReached refuses a connection past the limit.
ErrInvalidConnectionConfig refuses settings a connection cannot use.
*/
var (
	ErrNoVerifiedDomain = fault.New(fault.Invalid, "no_verified_domain",
		"Route a verified domain to this connection before activating it.").OnField("status")
	ErrHTTPSRequired = fault.New(fault.Invalid, "https_required",
		"Single sign-on needs this install to be served over HTTPS.").OnField("status")
	ErrConnectionEnforced = fault.New(fault.Conflict, "connection_enforced",
		"Stop enforcing this connection before deleting it.")
	ErrConnectionNotActive = fault.New(fault.Invalid, "invalid_transition",
		"Only an active connection can be enforced.").OnField("enforced")
	ErrEnforceUnproven = fault.New(fault.Invalid, "enforce_unproven",
		"Sign in through this connection, or test it and confirm owners can sign in, before enforcing it.").
		OnField("enforced")
	ErrConnectionLimitReached = fault.New(fault.RateLimited, "entitlement_limit_reached",
		"This organization has as many connections as it may have.")
	ErrInvalidConnectionConfig = fault.New(fault.Invalid, "invalid_saml_metadata",
		"The identity provider settings are incomplete or do not read.").OnField("config")
)

/*
Refusals of domains.

ErrDomainNotClaimable refuses a public suffix or a domain anyone can get
mail under. ErrDomainRoutesEnforced refuses to remove a domain routing to an
enforced connection. ErrDomainTaken refuses a domain another organization
verified. ErrDomainNotVerified refuses to route a domain still unproven.
ErrDomainLimitReached refuses a domain past the limit.
*/
var (
	ErrDomainNotClaimable = fault.New(fault.Invalid, "domain_not_claimable",
		"This domain cannot be claimed by one company.").OnField("domain")
	ErrDomainRoutesEnforced = fault.New(fault.Conflict, "domain_in_use",
		"This domain routes to an enforced connection. Stop enforcing it first.")
	ErrDomainTaken = fault.New(fault.Conflict, "domain_taken",
		"Another organization already verified this domain.")
	ErrDomainNotVerified = fault.New(fault.Invalid, "domain_not_verified",
		"Verify this domain before routing sign-ins through it.").OnField("sso_connection_id")
	ErrDomainLimitReached = fault.New(fault.RateLimited, "entitlement_limit_reached",
		"This organization has as many domains as it may have.")
)

/*
Refusals of a sign-in through a connection.

ErrSSOFailed answers any sign-in the identity provider's answer could not
complete, without saying why: the reason goes to the owners only.
ErrSSONoAccess refuses a person the connection may not admit.
ErrSSOEmailDomainNotVerified refuses an address outside the company's
verified domains routed to the connection.
*/
var (
	ErrSSOFailed = fault.New(fault.Invalid, "sso_failed",
		"Single sign-on did not complete. Try again or ask your administrator.")
	ErrSSONoAccess = fault.New(fault.Forbidden, "sso_no_access",
		"Your company's identity provider does not give you access to this workspace.")
	ErrSSOEmailDomainNotVerified = fault.New(fault.Forbidden, "sso_email_domain_not_verified",
		"Your address is not under a domain your company verified for single sign-on.")
)
