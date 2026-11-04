/*
|--------------------------------------------------------------------------
| Update Member
|--------------------------------------------------------------------------
|
| Every change to a member goes through one request: their role, whether
| they are active, and handing them ownership. All fields are checked
| against where the member stands before anything changes, and applied
| in one transaction, so a request either does everything it asks or
| nothing.
|
| Admins move members between member and admin and deactivate them.
| Anything touching an owner, such as promoting to owner, demoting one
| or deactivating one, takes an owner, through their own browser or
| app, who proved who they are within the last few minutes: a leaked
| token or an unattended laptop must not be enough to take a workspace
| over. A workspace never loses its last active owner. Changes to owners
| lock the workspace row first, so two owners demoting each other at the
| same moment cannot both succeed.
|
| Nobody deactivates themselves here; leaving a workspace is its own
| request.
|
*/

package members

import (
	"context"
	"errors"
	"fmt"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UpdateMemberInput is who changes a member and what changes.

Principal is the caller and Membership their membership of the
workspace. AssignedRole and Status are the requested values, nil to
leave as they are. ReassignTo names the member to hand a deactivated member's
open work to, and KeepOwner keeps the caller an owner when they make
someone else one.
*/
type UpdateMemberInput struct {
	Principal    *appauth.Principal
	Membership   authz.Membership
	MemberID     ids.UUID
	AssignedRole *enums.Role
	Status       *enums.MemberStatus
	ReassignTo   *ids.UUID
	KeepOwner    bool
}

/*
UpdateMember changes a member's role and status.

Rules:
  - Member and admin roles of non-owners need members:write.
  - Any change to or from the owner role needs members:manage_owners,
    the user's own browser or app, and a recent confirmation.
  - Deactivating or reactivating an owner needs members:manage_owners,
    anyone else members:write.
  - Members cannot deactivate themselves.
  - Ownership goes only to an active member who verified their email
    and has two-factor authentication where the workspace requires it;
    the caller becomes an admin unless KeepOwner.
  - A reactivated owner comes back as an admin unless a role is given.
  - The workspace always keeps an active owner: a change that would
    leave none is refused before the caller's power is weighed.
  - Values equal to the current ones change nothing.

Events: [appevents.MemberReactivated], [appevents.MemberRoleChanged]
and [appevents.MemberDeactivated], in that order, for what changed.

Errors: [exceptions.ErrNotFound], [exceptions.ErrFirstPartyOnly],
[exceptions.ErrForbidden], [fauth.ErrConfirmationRequired],
[exceptions.ErrInvalidTransition], [exceptions.ErrTargetUnverified],
[exceptions.ErrTargetTwoFactorRequired],
[exceptions.ErrReassignTargetInvalid], [exceptions.ErrLastOwner], or a
wrapped database error.
*/
type UpdateMember struct {
	DB            *database.DB
	Events        *events.Dispatcher
	Clock         clock.Clock
	ConfirmWindow time.Duration
}

/*
memberChange is what an update does to its member, worked out before
anything is written: whether they are reactivated and with which role
(comeBackAs), the role they end with, whether the caller steps down
from owner, and whether they are deactivated, handing their open work
to reassignTo.
*/
type memberChange struct {
	reactivate  bool
	comeBackAs  enums.Role
	changeRole  bool
	role        enums.Role
	demoteActor bool
	deactivate  bool
	reassignTo  ids.UUID
}

/*
subject is the workspace and member an update works on, both locked
for the transaction, and the queries of that transaction.
*/
type subject struct {
	q         *models.Queries
	workspace models.Workspace
	target    models.WorkspaceMember
}

/*
Handle applies the update and returns the member as they now are.
*/
func (a *UpdateMember) Handle(
	ctx context.Context, in UpdateMemberInput,
) (models.FindWorkspaceMemberWithUserRow, error) {
	var out models.FindWorkspaceMemberWithUserRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		s, err := lock(ctx, models.New(tx), in.Membership.Workspace.ID, in.MemberID)
		if err != nil {
			return err
		}

		change, err := a.plan(ctx, s, in)
		if err != nil {
			return err
		}

		w := memberWriter{tx: tx, events: a.Events, by: in.Membership.Member.ID, now: a.Clock.Now()}
		if err := w.apply(ctx, in.Membership.Member, s.target, change); err != nil {
			return err
		}

		out, err = findWithUser(ctx, s.q, s.target.WorkspaceID, s.target.ID)

		return err
	})

	return out, err
}

/*
lock locks the workspace, then the member, so changes to the members of
one workspace happen one after another.
*/
func lock(ctx context.Context, q *models.Queries, workspaceID, memberID ids.UUID) (subject, error) {
	workspace, err := q.LockWorkspace(ctx, workspaceID)
	if err != nil {
		return subject{}, fmt.Errorf("lock workspace: %w", err)
	}

	target, err := q.LockWorkspaceMember(ctx, models.LockWorkspaceMemberParams{WorkspaceID: workspaceID, ID: memberID})
	if errors.Is(err, database.ErrNotFound) {
		return subject{}, exceptions.ErrNotFound
	}

	if err != nil {
		return subject{}, fmt.Errorf("lock member: %w", err)
	}

	return subject{q: q, workspace: workspace, target: target}, nil
}

/*
plan checks the update against the caller's power and the member's
state, and works out what it changes.
*/
func (a *UpdateMember) plan(ctx context.Context, s subject, in UpdateMemberInput) (memberChange, error) {
	if err := a.admit(ctx, s, in); err != nil {
		return memberChange{}, err
	}

	change := memberChange{role: enums.Role(s.target.Role)}

	if in.Status != nil && string(*in.Status) != s.target.Status {
		if err := planStatus(&change, in, s.target); err != nil {
			return memberChange{}, err
		}
	}

	if in.AssignedRole != nil && *in.AssignedRole != change.role {
		if err := planRole(ctx, s, &change, in); err != nil {
			return memberChange{}, err
		}
	}

	if change.deactivate && in.ReassignTo != nil {
		if err := checkReassignee(ctx, s.q, s.target, *in.ReassignTo); err != nil {
			return memberChange{}, err
		}

		change.reassignTo = *in.ReassignTo
	}

	return change, nil
}

/*
admit refuses an update the caller's credential may not make, one that
would leave the workspace without an owner, and one the caller's power
does not cover, in that order.
*/
func (a *UpdateMember) admit(ctx context.Context, s subject, in UpdateMemberInput) error {
	if ownerRoleChange(in, s.target) && !in.Principal.IsFirstParty() {
		return exceptions.ErrFirstPartyOnly
	}

	if err := refuseLastOwner(ctx, s, in); err != nil {
		return err
	}

	return a.authorize(in, s.target)
}

/*
refuseLastOwner refuses, before the caller's power is weighed, a change
that would take the workspace's only active owner out of ownership. The
workspace row is locked by then, so of two owners demoting each other
at once the second is told the workspace needs its owner, whichever
role the first left them with.
*/
func refuseLastOwner(ctx context.Context, s subject, in UpdateMemberInput) error {
	if !appauth.IsOwnerRole(enums.Role(s.target.Role)) || s.target.Status != string(enums.MemberStatusActive) {
		return nil
	}

	demoted := in.AssignedRole != nil && !appauth.IsOwnerRole(*in.AssignedRole)
	deactivated := in.Status != nil && *in.Status == enums.MemberStatusDeactivated

	if !demoted && !deactivated {
		return nil
	}

	owners, err := s.q.CountActiveOwners(ctx, s.target.WorkspaceID)
	if err != nil {
		return fmt.Errorf("count owners: %w", err)
	}

	if owners <= 1 {
		return exceptions.ErrLastOwner
	}

	return nil
}

/*
ownerRoleChange reports whether the update sets the role of an owner,
or makes someone an owner.
*/
func ownerRoleChange(in UpdateMemberInput, target models.WorkspaceMember) bool {
	return in.AssignedRole != nil &&
		(appauth.IsOwnerRole(enums.Role(target.Role)) || appauth.IsOwnerRole(*in.AssignedRole))
}

/*
authorize checks that the caller may change each field asked for on
this member.
*/
func (a *UpdateMember) authorize(in UpdateMemberInput, target models.WorkspaceMember) error {
	if in.Status != nil && !in.Membership.Can(statusPermission(target)) {
		return exceptions.ErrForbidden
	}

	if in.AssignedRole == nil {
		return nil
	}

	if !ownerRoleChange(in, target) {
		if !in.Membership.Can("members:write") {
			return exceptions.ErrForbidden
		}

		return nil
	}

	if !in.Membership.Can("members:manage_owners") {
		return exceptions.ErrForbidden
	}

	if !fauth.Confirmed(in.Principal, a.Clock.Now(), a.ConfirmWindow) {
		return fauth.ErrConfirmationRequired
	}

	return nil
}

/*
planStatus works out a change of status.
*/
func planStatus(change *memberChange, in UpdateMemberInput, target models.WorkspaceMember) error {
	if *in.Status == enums.MemberStatusDeactivated {
		if target.ID == in.Membership.Member.ID {
			return transition("status", target.Status, string(*in.Status))
		}

		change.deactivate = true

		return nil
	}

	change.reactivate = true
	change.comeBackAs = change.role

	if appauth.IsOwnerRole(change.role) && in.AssignedRole == nil {
		change.comeBackAs = appauth.ManagerRole()
	}

	change.role = change.comeBackAs

	return nil
}

/*
planRole works out a change of role, handing ownership over when the
new role is owner.
*/
func planRole(ctx context.Context, s subject, change *memberChange, in UpdateMemberInput) error {
	if appauth.IsOwnerRole(*in.AssignedRole) {
		if in.Status != nil || s.target.Status != string(enums.MemberStatusActive) {
			return transition("role", s.target.Status, string(*in.AssignedRole))
		}

		if err := checkSuccessor(ctx, s.q, s.workspace, s.target); err != nil {
			return err
		}

		change.demoteActor = !in.KeepOwner && s.target.ID != in.Membership.Member.ID &&
			appauth.IsOwnerRole(enums.Role(in.Membership.Member.Role))
	}

	change.changeRole = true
	change.role = *in.AssignedRole

	return nil
}

/*
apply writes the planned change, in the order its events go out:
reactivation, role, the caller stepping down, deactivation. A change
that touched an owner must leave the workspace one.
*/
func (w memberWriter) apply(
	ctx context.Context, actor, target models.WorkspaceMember, c memberChange,
) error {
	member, err := w.applyRole(ctx, target, c)
	if err != nil {
		return err
	}

	if c.demoteActor {
		if err := w.stepDown(ctx, actor); err != nil {
			return err
		}
	}

	if c.deactivate {
		if err := w.deactivate(ctx, member, c.reassignTo); err != nil {
			return err
		}
	}

	if appauth.IsOwnerRole(enums.Role(target.Role)) || c.demoteActor {
		return w.keepsOwner(ctx, target.WorkspaceID)
	}

	return nil
}

/*
applyRole reactivates the member and gives them their new role, as
planned, and returns them as they then are.
*/
func (w memberWriter) applyRole(
	ctx context.Context, member models.WorkspaceMember, c memberChange,
) (models.WorkspaceMember, error) {
	var err error

	if c.reactivate {
		if member, err = w.reactivate(ctx, member, c.comeBackAs); err != nil {
			return member, err
		}
	}

	if c.changeRole {
		return w.changeRole(ctx, member, c.role)
	}

	return member, nil
}

/*
stepDown makes the caller an admin after they handed ownership over.
*/
func (w memberWriter) stepDown(ctx context.Context, actor models.WorkspaceMember) error {
	current, err := models.New(w.tx).LockWorkspaceMember(ctx, models.LockWorkspaceMemberParams{
		WorkspaceID: actor.WorkspaceID, ID: actor.ID,
	})
	if err != nil {
		return fmt.Errorf("lock caller's membership: %w", err)
	}

	_, err = w.changeRole(ctx, current, appauth.ManagerRole())

	return err
}

/*
statusPermission returns the permission needed to change the status of
the member, stricter for owners.
*/
func statusPermission(target models.WorkspaceMember) string {
	if appauth.IsOwnerRole(enums.Role(target.Role)) {
		return "members:manage_owners"
	}

	return "members:write"
}

/*
transition returns the refusal of a move from one state to another,
blamed on the field.
*/
func transition(field, from, to string) error {
	return exceptions.ErrInvalidTransition.OnField(field).With("from", from).With("to", to)
}
