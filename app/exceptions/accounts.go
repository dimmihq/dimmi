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
		"Too many attempts. Try again later.")
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
Mailed secret failures.

A link or code that is unknown, wrong, or of another kind fails the
same way as one that never existed, so a guess learns nothing.
ErrInvalidResetToken answers every password reset that cannot be used.
ErrInvalidVerificationToken answers an email verification that does
not match. ErrInvalidCode answers a wrong or spent code.
ErrMagicLinkExpired answers a sign-in link that was used, ran out, or
never existed. ErrVerificationLinkStale answers a verification sent to
an address the account no longer has, and ErrVerificationLinkExpired
one that ran out before the account was verified.
*/
var (
	ErrInvalidResetToken = fault.New(fault.Invalid, "invalid_reset_token",
		"This password reset link is invalid or has expired.").OnField("token")
	ErrInvalidVerificationToken = fault.New(fault.Invalid, "invalid_verification_token",
		"This verification link is invalid.").OnField("token")
	ErrInvalidCode = fault.New(fault.Invalid, "invalid_code",
		"The code is invalid or has expired.").OnField("code")
	ErrMagicLinkExpired = fault.New(fault.Gone, "magic_link_expired",
		"This sign-in link was already used or has expired. Request a new one.")
	ErrVerificationLinkStale = fault.New(fault.Gone, "verification_link_stale",
		"This verification link was sent to an address the account no longer uses.")
	ErrVerificationLinkExpired = fault.New(fault.Gone, "verification_link_expired",
		"This verification link has expired. Sign in and ask for a new one.")
)

/*
Confirmation failures.

ErrMethodNotAvailable refuses a way of confirming that the account
cannot use, such as a password on an account without one; it is not a
guess and counts as none. ErrWrongPassword answers a wrong password
while confirming.
*/
var (
	ErrMethodNotAvailable = fault.New(fault.Invalid, "method_not_available",
		"This method is not available for your account.").OnField("method")
	ErrWrongPassword = fault.New(fault.Invalid, "invalid_password",
		"The password is incorrect.").OnField("password")
)

/*
ErrEmailUnverified refuses a step that reaches other people, such as
inviting them, until the user proved their email.
*/
var ErrEmailUnverified = fault.New(fault.Forbidden, "email_unverified", "Verify your email address to continue.")

/*
Identity provider failures.

ErrOAuthEmailUnverified refuses a first sign-in through a provider that
does not vouch for the person's address, since the address would decide
which account they get. ErrIdentityTaken refuses to link a provider
account another user already linked. ErrProviderAlreadyLinked refuses a
second account of a provider for a user who has one linked.
ErrLastLoginMethod refuses to unlink the only way left to sign in.
*/
var (
	ErrOAuthEmailUnverified = fault.New(fault.Invalid, "oauth_email_unverified",
		"The provider has not verified your email address.")
	ErrIdentityTaken = fault.New(fault.Conflict, "identity_taken",
		"This account is already linked to another user.")
	ErrProviderAlreadyLinked = fault.New(fault.Conflict, "provider_already_linked",
		"You already linked an account of this provider.")
	ErrLastLoginMethod = fault.New(fault.Conflict, "last_login_method",
		"This is your only way to sign in. Set a password or link another account first.")
)
