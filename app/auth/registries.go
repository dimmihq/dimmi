/*
|--------------------------------------------------------------------------
| Registry Values
|--------------------------------------------------------------------------
|
| Some columns hold values that grow with the product rather than with
| the schema: how someone signed in, how an account was created, what a
| security event records. The database only checks their shape, so a
| new sign-in provider needs no migration; this file is the list the
| application accepts. A value that is not registered here is refused by
| request rules and actions before it can reach a row.
|
*/

package auth

import "slices"

/*
Registry is a named set of accepted values for one registry column.
*/
type Registry struct {
	name   string
	values []string
}

/*
Values returns the accepted values, in registration order.
*/
func (r Registry) Values() []string {
	return slices.Clone(r.values)
}

/*
Has reports whether the value is registered.
*/
func (r Registry) Has(value string) bool {
	return slices.Contains(r.values, value)
}

/*
The ways a person proves who they are. Primary methods sign a person in
alone; second factors only follow one. Provider keys such as github are
primary methods too.
*/
const (
	MethodPassword      = "password"
	MethodMagicLink     = "magic_link"
	MethodEmailCode     = "email_code"
	MethodPasswordReset = "password_reset"
	MethodSetup         = "setup"
	MethodInvitation    = "invitation"
	MethodGitHub        = "github"
	MethodGoogle        = "google"
	MethodTOTP          = "totp"
	MethodRecoveryCode  = "recovery_code"
)

/*
How an account was created and how a member joined a workspace.
*/
const (
	CreatedViaRegister  = "register"
	CreatedViaMagicLink = MethodMagicLink
	JoinedViaCreated    = "created"
)

/*
The security events written so far.
*/
const (
	EventLoginSucceeded        = "login.succeeded"
	EventLoginFailed           = "login.failed"
	EventLoginLocked           = "login.locked"
	EventLogout                = "logout.succeeded"
	EventSessionRevoked        = "session.revoked"
	EventSessionsRevokedOthers = "sessions.revoked_others"
	EventPasswordReset         = "password.reset"
	EventMagicLinkUsed         = "magic_link.used"
)

/*
PrimaryMethods lists the primary sign-in methods a session can record.
*/
func PrimaryMethods() Registry {
	return Registry{name: "auth_methods", values: []string{
		MethodPassword, MethodMagicLink, MethodEmailCode, MethodPasswordReset, MethodSetup, MethodInvitation,
		MethodGitHub, MethodGoogle,
	}}
}

/*
SecondFactors lists the methods that complete a sign-in after a primary
one.
*/
func SecondFactors() Registry {
	return Registry{name: "second_factors", values: []string{MethodTOTP, MethodRecoveryCode}}
}

/*
CreatedVia lists how an account can come to exist.
*/
func CreatedVia() Registry {
	return Registry{name: "created_via", values: []string{
		CreatedViaRegister, MethodMagicLink, MethodInvitation, MethodSetup, "cli", MethodGitHub, MethodGoogle,
	}}
}

/*
JoinedVia lists how a member can join a workspace.
*/
func JoinedVia() Registry {
	return Registry{name: "joined_via", values: []string{JoinedViaCreated, MethodInvitation, MethodSetup, "cli"}}
}

/*
SecurityEvents lists the entries of a user's own security log.
*/
func SecurityEvents() Registry {
	return Registry{name: "security_events", values: []string{
		EventLoginSucceeded, EventLoginFailed, EventLoginLocked, EventLogout, "password.changed", EventPasswordReset,
		"two_factor.enabled", "two_factor.disabled", "auth_factor.added", "auth_factor.removed",
		"auth_factor.renamed", "auth_factor.recovery_code_used", "auth_factor.recovery_codes_regenerated",
		"email.change_requested", "email.changed", "identity.linked", "identity.unlinked", EventSessionRevoked,
		EventSessionsRevokedOthers, EventMagicLinkUsed, "token.created", "account.deletion_requested",
	}}
}

/*
Registries returns every registry, keyed by name, for the request rule
that checks a field against one of them.
*/
func Registries() map[string]Registry {
	all := map[string]Registry{}
	for _, r := range []Registry{PrimaryMethods(), SecondFactors(), CreatedVia(), JoinedVia(), SecurityEvents()} {
		all[r.name] = r
	}

	return all
}
