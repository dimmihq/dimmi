/*
|--------------------------------------------------------------------------
| Access Rules
|--------------------------------------------------------------------------
|
| The rules a workspace may set, checked in a fixed order on every request
| inside it: where the caller connects from, whether they came through
| the company's identity provider, how they signed in, how long ago, and
| whether a second factor protects them. The first one failed is the one
| the person is asked to fix.
|
| Sign-in method, domain and session length judge how a person signed in,
| so they bind browsers and apps and never access tokens: a token is not a
| sign-in. The allowlists are split the same way, one list for people and
| one for programs, since an office network and a CI runner have nothing
| in common.
|
*/

package auth

import (
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The requirements a refusal can name.
*/
const (
	RequirementIPNotAllowed     = "ip_not_allowed"
	RequirementSSORequired      = "sso_required"
	RequirementSignInMethod     = "sign_in_method"
	RequirementReauthentication = "reauthentication_required"
)

/*
SignInAgainPage is where a person signs in again on the session they
already have.
*/
const SignInAgainPage = "/login?reauth=1"

/*
Why a session is asked to sign in again.
*/
const (
	ReauthReasonMaxAge = "max_age"
	ReauthReasonIdle   = "idle"
)

/*
MethodSSOBypass marks a sign-in where an owner stepped around their
company's identity provider. It is recorded beside the methods passed,
never instead of them.
*/
const MethodSSOBypass = "sso_bypass"

/*
MethodGoogle is the method of a session signed in through Google, the
one provider whose accounts can answer for a company domain.
*/
const MethodGoogle = "google"

/*
PolicyMethod returns the name a workspace's rule knows a session's
primary method by: a mailed code is a magic link and a reset a password.
*/
func PolicyMethod(method string) string {
	switch method {
	case MethodEmailCode:
		return MethodMagicLink
	case MethodPasswordReset:
		return MethodPassword
	default:
		return method
	}
}

/*
SignInAllowed reports whether the session's primary method is on the
workspace's list. Accepting an invitation or finishing setup proves the
same mailbox a magic link does and sets the password a sign-in then
uses, so it passes where either of those is allowed.
*/
func SignInAllowed(method string, allowed []string) bool {
	if method == MethodInvitation || method == MethodSetup {
		return slices.Contains(allowed, MethodPassword) || slices.Contains(allowed, MethodMagicLink)
	}

	return slices.Contains(allowed, PolicyMethod(method))
}

/*
Bypassed reports whether an owner signed in around the identity
provider. Only an owner's sign-in counts; the record on anyone else's
exempts them from nothing.
*/
func (a AccessCheck) Bypassed() bool {
	return IsOwnerRole(a.Role) && slices.Contains(a.Principal.SignIn.Methods, MethodSSOBypass)
}

/*
AddressRule refuses a caller connecting from outside the list that binds
their kind of credential: the dashboard list for browsers and apps, the
API list for tokens.
*/
type AddressRule struct{}

/*
Check refuses an address outside the list.
*/
func (AddressRule) Check(a AccessCheck) error {
	list := a.Settings.DashboardAllowlist
	if !a.Principal.IsFirstParty() {
		list = a.Settings.APIAllowlist
	}

	if models.AddressAllowed(list, a.Principal.IP) {
		return nil
	}

	address := ""
	if a.Principal.IP.IsValid() {
		address = a.Principal.IP.Unmap().String()
	}

	return exceptions.ErrIPNotAllowed.
		With("requirement", RequirementIPNotAllowed).
		With("action_url", nil).
		With("ip", address)
}

/*
SSORule refuses a browser or app that did not sign in through one of the
connections the organization enforces. A session through another
connection, even of the same organization, does not count. Redirect is
where a sign-in through a connection starts.
*/
type SSORule struct {
	Redirect func(connection ids.UUID) string
}

/*
Check refuses a session outside the enforced connections.
*/
func (r SSORule) Check(a AccessCheck) error {
	enforced := a.Settings.SSOEnforcedConnectionIDs
	if len(enforced) == 0 || !a.Principal.IsFirstParty() || a.Bypassed() {
		return nil
	}

	if slices.Contains(enforced, a.Principal.SSOConnectionID) {
		return nil
	}

	url := r.Redirect(enforced[0])

	return exceptions.ErrSSORequired.
		With("requirement", RequirementSSORequired).
		With("action_url", url+"?intent=link").
		With("redirect_url", url)
}

/*
SignInMethodRule refuses a browser or app whose sign-in method the
workspace does not allow. A second factor never counts: a password and a
passkey is a password.
*/
type SignInMethodRule struct{}

/*
Check refuses a method off the list.
*/
func (SignInMethodRule) Check(a AccessCheck) error {
	allowed := a.Settings.AllowedSignInMethods
	if len(allowed) == 0 || !a.Principal.IsFirstParty() || a.Bypassed() {
		return nil
	}

	if SignInAllowed(a.Principal.SignIn.Method, allowed) {
		return nil
	}

	return exceptions.ErrSignInMethodRequired.
		With("requirement", RequirementSignInMethod).
		With("action_url", SignInAgainPage).
		With("allowed_methods", allowed)
}

/*
GoogleDomainRule refuses a Google sign-in from a domain the workspace
does not list. A personal Google account has no domain at all, so it
never passes while a list is set.
*/
type GoogleDomainRule struct{}

/*
Check refuses a Google sign-in from another domain.
*/
func (GoogleDomainRule) Check(a AccessCheck) error {
	allowed := a.Settings.GoogleAllowedDomains
	if len(allowed) == 0 || !a.Principal.IsFirstParty() || a.Principal.SignIn.Method != MethodGoogle {
		return nil
	}

	if slices.Contains(allowed, a.Principal.AuthDomain) {
		return nil
	}

	return exceptions.ErrSignInMethodRequired.
		With("requirement", RequirementSignInMethod).
		With("action_url", SignInAgainPage).
		With("allowed_domains", allowed)
}

/*
SessionLengthRule asks a browser or app to sign in again when the last
full sign-in is older than the workspace allows, or nothing has happened
in a workspace for longer than it allows. It only ever shortens what the
install allows, and binds this workspace alone, so one strict workspace
never ends a person's work in another.
*/
type SessionLengthRule struct{}

/*
Check refuses a session past either limit.
*/
func (SessionLengthRule) Check(a AccessCheck) error {
	if !a.Principal.IsFirstParty() {
		return nil
	}

	if over(a.Now, a.Principal.AuthenticatedAt, a.Settings.SessionMaxAge) {
		return reauthentication(ReauthReasonMaxAge)
	}

	if over(a.Now, a.Principal.LastActivityAt, a.Settings.SessionIdleTimeout) {
		return reauthentication(ReauthReasonIdle)
	}

	return nil
}

/*
over reports whether more minutes than the limit passed since the time;
no limit is never over.
*/
func over(now, since time.Time, limit *int) bool {
	return limit != nil && now.Sub(since) > time.Duration(*limit)*time.Minute
}

/*
reauthentication is the refusal asking a person to sign in again.
*/
func reauthentication(reason string) error {
	return exceptions.ErrReauthenticationRequired.
		With("requirement", RequirementReauthentication).
		With("action_url", SignInAgainPage).
		With("reason", reason)
}
