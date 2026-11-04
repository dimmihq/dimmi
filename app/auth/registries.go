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
alone; second factors only follow one. The key of every registered
identity provider, such as github, is a primary method too.
*/
const (
	MethodPassword      = "password"
	MethodMagicLink     = "magic_link"
	MethodEmailCode     = "email_code"
	MethodPasswordReset = "password_reset"
	MethodSetup         = "setup"
	MethodInvitation    = "invitation"
	MethodTOTP          = "totp"
	MethodRecoveryCode  = "recovery_code"
	MethodPasskey       = "passkey"
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
	EventPasswordChanged       = "password.changed"
	EventPasswordReset         = "password.reset"
	EventEmailChangeRequested  = "email.change_requested"
	EventEmailChanged          = "email.changed"
	EventMagicLinkUsed         = "magic_link.used"
	EventIdentityLinked        = "identity.linked"
	EventIdentityUnlinked      = "identity.unlinked"
	EventTokenCreated          = "token.created"
	EventTokenRevoked          = "token.revoked"
	EventTwoFactorResetAsked   = "two_factor.reset_requested"
	EventTwoFactorResetDone    = "two_factor.reset_completed"
)

/*
PrimaryMethods lists the primary sign-in methods a session can record,
the keys of the identity providers included.
*/
func PrimaryMethods(providers []string) Registry {
	return Registry{name: "auth_methods", values: slices.Concat([]string{
		MethodPassword, MethodMagicLink, MethodEmailCode, MethodPasswordReset, MethodSetup, MethodInvitation,
	}, providers)}
}

/*
SecondFactors lists the methods that complete a sign-in after a primary
one.
*/
func SecondFactors() Registry {
	return Registry{name: "second_factors", values: []string{MethodTOTP, MethodPasskey, MethodRecoveryCode}}
}

/*
CreatedVia lists how an account can come to exist, signing up through
any of the identity providers included.
*/
func CreatedVia(providers []string) Registry {
	return Registry{name: "created_via", values: slices.Concat([]string{
		CreatedViaRegister, MethodMagicLink, MethodInvitation, MethodSetup, "cli",
	}, providers)}
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
		EventLoginSucceeded, EventLoginFailed, EventLoginLocked, EventLogout, EventPasswordChanged, EventPasswordReset,
		"two_factor.enabled", "two_factor.disabled", "auth_factor.added", "auth_factor.removed",
		"auth_factor.renamed", "auth_factor.recovery_code_used", "auth_factor.recovery_codes_regenerated",
		EventEmailChangeRequested, EventEmailChanged, EventIdentityLinked, EventIdentityUnlinked, EventSessionRevoked,
		EventSessionsRevokedOthers, EventMagicLinkUsed, EventTokenCreated, EventTokenRevoked,
		"account.deletion_requested", EventTwoFactorResetAsked, EventTwoFactorResetDone,
	}}
}

/*
Registries returns every registry, keyed by name, for the request rule
that checks a field against one of them. Providers are the keys of the
registered identity providers.
*/
func Registries(providers []string) map[string]Registry {
	all := map[string]Registry{}
	for _, r := range []Registry{
		PrimaryMethods(providers), SecondFactors(), CreatedVia(providers), JoinedVia(), SecurityEvents(),
	} {
		all[r.name] = r
	}

	return all
}
