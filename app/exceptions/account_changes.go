/*
|--------------------------------------------------------------------------
| Account Change Failures
|--------------------------------------------------------------------------
|
| What a person can get wrong changing their own account. A link to move
| to a new address that cannot be used always fails the same way, so a
| guess learns nothing, while an address someone else took meanwhile is
| said openly: the person holding that link proved they own the mailbox.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Account change failures.

ErrInvalidEmailChangeToken answers every link to a new address that
cannot be used. ErrEmailChangeTaken means another account took the new
address before the link was opened; the change is dropped.
ErrEmailUnchanged refuses a change to the address the account has.
ErrCurrentPasswordRequired and ErrWrongCurrentPassword refuse a new
password without the old one. ErrInvalidAvatar refuses a file that is
not a picture of an accepted kind, and ErrAvatarTooLarge one over the
limit. ErrSoleOwner refuses to delete an account that alone owns a
workspace other people work in, and ErrConfirmEmailMismatch one whose
address was not typed correctly.
*/
var (
	ErrInvalidEmailChangeToken = fault.New(fault.Invalid, "invalid_email_change_token",
		"This confirmation link is invalid or has expired.").OnField("token")
	ErrEmailChangeTaken = fault.New(fault.Conflict, "email_taken", "The email has already been taken.")
	ErrEmailUnchanged   = fault.New(fault.Invalid, "email_unchanged",
		"This is already your email address.").OnField("email")
	ErrCurrentPasswordRequired = fault.New(fault.Invalid, "current_password_required",
		"Enter your current password.").OnField("current_password")
	ErrWrongCurrentPassword = fault.New(fault.Invalid, "invalid_password",
		"The password is incorrect.").OnField("current_password")
	ErrInvalidAvatar = fault.New(fault.Invalid, "invalid_avatar",
		"Choose a PNG, JPEG, WebP or GIF picture.").OnField("avatar")
	ErrAvatarTooLarge = fault.New(fault.Invalid, "avatar_too_large",
		"The picture must be 2 MB or smaller.").OnField("avatar")
	ErrSoleOwner = fault.New(fault.Conflict, "sole_owner",
		"Hand over or delete the workspaces you alone own first.")
	ErrConfirmEmailMismatch = fault.New(fault.Invalid, "confirm_email_mismatch",
		"Type your email address to confirm.").OnField("confirm_email")
)
