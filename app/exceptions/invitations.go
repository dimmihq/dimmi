/*
|--------------------------------------------------------------------------
| Invitation Errors
|--------------------------------------------------------------------------
|
| What can go wrong while inviting people and answering an invitation.
| An invitation link is a secret of its own, so an unknown one is simply
| not found, while the holder of a real one learns where it stands:
| expired, withdrawn or already used. An invitation belongs to the
| address it was sent to; nobody signed in as someone else may use it.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Inviting failures.

ErrInvalidInvitationEmail marks one address of a batch that is not an
email address; the rest of the batch goes ahead. ErrInvitationLimitReached
refuses a batch that would send more invitations than the workspace may
send in a day, and carries retry_after.
*/
var (
	ErrInvalidInvitationEmail = fault.New(fault.Invalid, "invalid_email",
		"The email must be a valid email address.").OnField("email")
	ErrInvitationLimitReached = fault.New(fault.RateLimited, "invitation_limit_reached",
		"This workspace sent as many invitations as it may today. Try again tomorrow.")
)

/*
Answering failures.

ErrInvitationEmailMismatch refuses an invitation to an address other
than the caller's. ErrInvitationExpired and ErrInvitationRevoked answer
a link that ran out or was withdrawn; an admin can send a new one.
ErrInvitationAlreadyAccepted answers a link that was used.
ErrAlreadyMember refuses an invitation to someone already active in the
workspace.
*/
var (
	ErrInvitationEmailMismatch = fault.New(fault.Forbidden, "invitation_email_mismatch",
		"This invitation was sent to another email address. Sign in with that address to accept it.")
	ErrInvitationExpired = fault.New(fault.Gone, "invitation_expired",
		"This invitation has expired. Ask for a new one.")
	ErrInvitationRevoked = fault.New(fault.Gone, "invitation_revoked",
		"This invitation was withdrawn.")
	ErrInvitationAlreadyAccepted = fault.New(fault.Conflict, "invitation_already_accepted",
		"This invitation was already accepted.")
	ErrAlreadyMember = fault.New(fault.Conflict, "already_member",
		"You are already a member of this workspace.")
)
