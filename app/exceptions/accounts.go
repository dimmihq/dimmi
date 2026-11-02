/*
|--------------------------------------------------------------------------
| Account Errors
|--------------------------------------------------------------------------
|
| What can go wrong while signing up, signing in and managing sessions.
| Codes never change once shipped: apps and scripts branch on them. A
| failed sign-in always says the same thing whether the email exists or
| not, so the answer never tells an attacker which accounts exist.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Sign-in failures.

ErrInvalidCredentials answers a wrong password and an unknown email
alike. ErrTooManyAttempts answers every locked sign-in the same way, and
carries retry_after. ErrAuthBusy means the server cannot verify a
password right now, because too many are being checked at once or the
sign-in policy could not be read; nothing was verified and the client
should retry. ErrReauthUserMismatch means a fresh sign-in on an existing
session proved someone other than the session's user.
*/
var (
	ErrInvalidCredentials = fault.New(fault.Invalid, "invalid_credentials",
		"These credentials do not match our records.").OnField("email")
	ErrTooManyAttempts = fault.New(fault.RateLimited, "too_many_attempts",
		"Too many login attempts. Try again later.")
	ErrAuthBusy = fault.New(fault.Unavailable, "auth_busy",
		"The server is busy. Try again in a moment.").With("retry_after", 1)
	ErrReauthUserMismatch = fault.New(fault.Invalid, "reauth_user_mismatch",
		"Sign in as the user of this session.").OnField("email")
	ErrGrantNotAllowed = fault.New(fault.Invalid, "grant_not_allowed",
		"This way of signing in is not available.").OnField("grant")
)

/*
Signup failures.

ErrSignupClosed means the install does not take new accounts and the
person holds no invitation. ErrEmailTaken means an account already uses
the address; registration says so openly, and is throttled instead.
ErrTooManySignups caps the accounts one address creates per hour.
*/
var (
	ErrSignupClosed = fault.New(fault.Forbidden, "signup_closed", "Signing up is closed on this server.")
	ErrEmailTaken   = fault.New(fault.Invalid, "email_taken",
		"The email has already been taken.").OnField("email")
	ErrTooManySignups = fault.New(fault.RateLimited, "too_many_attempts",
		"Too many accounts were created from your address. Try again later.")
)

/*
Password rule failures, reported on the password field.

ErrPasswordTooCommon means the password is on the list of most used
passwords, or is the account's own email or name.
ErrPasswordCompromised means it appeared in a known data breach.
*/
var (
	ErrPasswordTooCommon = fault.New(fault.Invalid, "password_too_common",
		"This password is too common. Choose another one.").OnField("password")
	ErrPasswordCompromised = fault.New(fault.Invalid, "password_compromised",
		"This password appeared in a data breach. Choose another one.").OnField("password")
)

/*
Credential failures.

ErrFirstPartyOnly refuses a personal access token or third-party OAuth
token on a route that only the user's own browser or app may call, so a
leaked token can never change the account or mint more tokens.
ErrSessionRequired refuses a bearer token on a browser-only route.
*/
var (
	ErrFirstPartyOnly = fault.New(fault.Forbidden, "first_party_only",
		"This route is only available to the signed-in user, not to access tokens.")
	ErrSessionRequired = fault.New(fault.Forbidden, "session_required", "This route needs a browser session.")
)

/*
Lookup failures.

ErrNotFound answers any id the caller may not see, including other
users' and other workspaces' ids, so a guess never confirms that a row
exists. ErrWorkspaceNotFound does the same for workspaces.
*/
var (
	ErrNotFound          = fault.New(fault.NotFound, "not_found", "Not Found.")
	ErrWorkspaceNotFound = fault.New(fault.NotFound, "workspace_not_found", "Workspace not found.")
)

/*
ErrSSORequired means the sign-in policy for the address does not allow
the method used, such as a password where the organization requires
single sign-on. It answers for the whole domain, so it says nothing
about whether an account exists.
*/
var ErrSSORequired = fault.New(fault.Conflict, "sso_required", "Sign in with your organization's single sign-on.")

/*
Workspace access failures.

ErrMembershipDeactivated refuses a workspace the caller was a member of
but no longer is active in. ErrWorkspaceMismatch refuses a request that
names a workspace other than the one its token is bound to.
*/
var (
	ErrMembershipDeactivated = fault.New(fault.Forbidden, "membership_deactivated",
		"Your membership of this workspace is not active.")
	ErrWorkspaceMismatch = fault.New(fault.Forbidden, "workspace_mismatch",
		"This token belongs to another workspace.")
)

/*
ErrTwoFactorUnavailable refuses to sign in an account protected by a
second factor on a server that cannot yet ask for it. Starting a session
without the second factor would silently weaken the account.
*/
var ErrTwoFactorUnavailable = fault.New(fault.Unavailable, "two_factor_unavailable",
	"This account uses two-factor authentication, which this server cannot check yet.")
