/*
|--------------------------------------------------------------------------
| Sign-In Help Failures
|--------------------------------------------------------------------------
|
| What can stop an admin from helping a teammate back in, and the
| teammate from finishing it. An admin of one company never weakens an
| account that also works for another, so that case is refused with
| the way out the teammate still has. A reset link that cannot be used
| always fails the same way, so a guess learns nothing.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Sign-in help failures.

ErrResetSelf refuses an admin helping themselves; they use their own
recovery codes. ErrResetTargetInactive refuses a member who no longer
works in the workspace. ErrResetTargetUnverified refuses a member who
never proved their address, where the link would go. ErrResetBlocked
refuses a member who also works for another organization.
ErrInvalidTwoFactorResetToken answers every reset link that cannot be
used, and ErrResetPasswordRequired asks for the account's password with
the link.
*/
var (
	ErrResetSelf = fault.New(fault.Invalid, "cannot_reset_self",
		"Use your own recovery codes to get back in.").OnField("member")
	ErrResetTargetInactive = fault.New(fault.Conflict, "member_inactive",
		"Only an active member can be helped.")
	ErrResetTargetUnverified = fault.New(fault.Conflict, "target_unverified",
		"This member has not verified their email address, so no link can reach them.")
	ErrResetBlocked = fault.New(fault.Conflict, "two_factor_reset_blocked",
		"Ask them to use recovery codes or contact support.")
	ErrInvalidTwoFactorResetToken = fault.New(fault.Invalid, "invalid_two_factor_reset_token",
		"This reset link is invalid or has expired.").OnField("token")
	ErrResetPasswordRequired = fault.New(fault.Invalid, "password_required",
		"Enter your password to turn two-factor authentication off.").OnField("password")
)
