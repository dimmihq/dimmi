/*
|--------------------------------------------------------------------------
| Widget Session Failures
|--------------------------------------------------------------------------
|
| What a visitor's messenger hears when its session or a request on it is
| refused. Each code tells the messenger what to do next: drop the token and
| start over, ask the page for a fresh identity, or show the visitor a field,
| so the codes stay stable once a messenger reads them.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Failures of a visitor's session.
*/
var (
	ErrSessionInvalid = fault.New(fault.Unauthenticated, "session_invalid",
		"This messenger session is not known.")
	ErrSessionExpired = fault.New(fault.Unauthenticated, "session_expired",
		"This messenger session has expired.")
	ErrSessionRevoked = fault.New(fault.Unauthenticated, "session_revoked",
		"This messenger session has ended.")
	ErrIdentityProofRequired = fault.New(fault.Conflict, "identity_proof_required",
		"A signed-in session resumes only with a fresh identity token.")
	ErrIdentityMismatch = fault.New(fault.Conflict, "identity_mismatch",
		"The identity token names someone else; start a new session.")
	ErrIdentityLocked = fault.New(fault.Forbidden, "identity_locked",
		"A signed-in customer's profile comes from your app, not the messenger.")
	ErrIdentityNotConfigured = fault.New(fault.Unauthenticated, "identity_not_configured",
		"This messenger does not verify signed-in customers yet.")
)

/*
Failures of a visitor's conversations and messages.
*/
var (
	ErrWidgetConversationNotFound = fault.New(fault.NotFound, "conversation_not_found",
		"No conversation of yours has this id.")
	ErrEmailRequired = fault.New(fault.Invalid, "email_required",
		"Leave an email so the team can reply while they are away.")
	ErrAttachmentNotUploaded = fault.New(fault.Conflict, "attachment_not_uploaded",
		"Every attached file must finish uploading first.")
	ErrReadStateForeignMessage = fault.New(fault.Invalid, "message_not_in_conversation",
		"That message is not in this conversation.")
	ErrAttachmentsDisabled = fault.New(fault.Forbidden, "attachments_disabled",
		"This messenger does not take files.")
)
