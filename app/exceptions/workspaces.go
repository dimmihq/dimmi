/*
|--------------------------------------------------------------------------
| Workspace And Member Errors
|--------------------------------------------------------------------------
|
| What can go wrong while working inside a workspace and changing who
| belongs to it. A workspace always keeps someone who can run it, so
| every change that would leave it without an active owner is refused,
| and a change of state that makes no sense from where a member stands
| says which move was asked for.
|
*/

package exceptions

import (
	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
)

/*
Workspace access failures.

ErrWorkspaceRequired asks a browser or app whose user belongs to several
workspaces to name one in the Dimmi-Workspace header; it is a malformed
request rather than invalid input. ErrWorkspaceSuspended refuses a
change to a workspace an operator suspended; reading it still works.
ErrForbidden refuses an action the caller's role does not allow.
*/
var (
	ErrWorkspaceRequired = &fhttp.ResponseError{
		Status: fhttp.StatusBadRequest, Code: "workspace_required",
		Message: "Name the workspace in the Dimmi-Workspace header.",
	}
	ErrWorkspaceSuspended = fault.New(fault.Forbidden, "workspace_suspended",
		"This workspace is suspended. It can be read but not changed.")
	ErrForbidden = fault.New(fault.Forbidden, "forbidden", "This action is unauthorized.")
)

/*
Workspace settings failures.

ErrSlugReserved refuses a slug that reads as part of the product or that
another workspace gave up recently. ErrSlugTaken refuses one another
workspace uses. ErrTestModeImmutable refuses to turn a live workspace
into a test one, or back, after it was created.
*/
var (
	ErrSlugReserved = fault.New(fault.Invalid, "slug_reserved",
		"This address is reserved. Choose another one.").OnField("slug")
	ErrSlugTaken = fault.New(fault.Invalid, "slug_taken",
		"Another workspace already uses this address.").OnField("slug")
	ErrTestModeImmutable = fault.New(fault.Invalid, "test_mode_immutable",
		"Test mode is chosen when a workspace is created and cannot change.").OnField("test_mode")
)

/*
Workspace deletion failures.

ErrConfirmSlugMismatch refuses a deletion whose confirmation does not
spell the workspace's address, so a workspace is never deleted by a
misdirected click. ErrWorkspacePurging refuses to restore a workspace
whose purge has begun or whose time in the trash ran out.
*/
var (
	ErrConfirmSlugMismatch = fault.New(fault.Invalid, "confirm_slug_mismatch",
		"Type the workspace's address to confirm.").OnField("confirm_slug")
	ErrWorkspacePurging = fault.New(fault.Gone, "workspace_purging",
		"This workspace is being deleted for good and can no longer be restored.")
)

/*
Member failures.

ErrLastOwner refuses to demote, deactivate or remove the last active
owner of a workspace; ownership must be handed over first.
ErrInvalidTransition refuses a change of state, of a member or an
invitation, that cannot happen from where it stands; callers add the
from and to states.
ErrTargetUnverified and ErrTargetTwoFactorRequired refuse to hand
ownership to a member who has not proved their email, or who lacks the
second factor the workspace requires. ErrReassignTargetInvalid refuses
to hand a removed member's open work to someone who cannot take it.
*/
var (
	ErrLastOwner = fault.New(fault.Conflict, "last_owner",
		"A workspace needs an active owner. Make someone else an owner first.")
	ErrInvalidTransition = fault.New(fault.Invalid, "invalid_transition",
		"This change is not possible from the current state.")
	ErrTargetUnverified = fault.New(fault.Conflict, "target_unverified",
		"Ownership can only go to a member who verified their email address.")
	ErrTargetTwoFactorRequired = fault.New(fault.Conflict, "target_two_factor_required",
		"This workspace requires two-factor authentication, which the member has not set up.")
	ErrReassignTargetInvalid = fault.New(fault.Invalid, "reassign_target_invalid",
		"Open work can only go to another active member of this workspace.").OnField("reassign_to_member_id")
)
