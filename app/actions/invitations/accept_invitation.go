/*
|--------------------------------------------------------------------------
| Accept Invitation
|--------------------------------------------------------------------------
|
| Whoever holds an invitation link may see what it invites them to, and
| whether an account already uses the address, so the page can ask them
| to sign in or to sign up. The address itself stays masked: the link
| may have been forwarded.
|
| An invitation is accepted by the account with the address it was sent
| to, signed in, and by nobody else. Following the link proves the
| person reads mail at that address, so an unverified address becomes
| verified. Someone who was deactivated gets their old membership back,
| with its history, in the role the invitation offers; anyone else gets
| a new one. Teams of the invitation that still exist are joined, and
| whoever sent it hears about it while they are still in the workspace.
|
*/

package invitations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	fmail "github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Preview is what an invitation link invites to: the invitation as its
holder may see it, its status as of now, and whether an account uses
its address.
*/
type Preview struct {
	Invitation models.PreviewInvitationRow
	Status     enums.InvitationStatus
	HasAccount bool
}

/*
PreviewInvitation shows an invitation to whoever holds its link.

Errors: [exceptions.ErrNotFound] for a link of no invitation, or of a
workspace in the trash, or a wrapped database error.
*/
type PreviewInvitation struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle returns the invitation the token belongs to.
*/
func (a *PreviewInvitation) Handle(ctx context.Context, token string) (Preview, error) {
	q := models.New(a.DB)

	inv, err := findByToken(ctx, q, token)
	if err != nil {
		return Preview{}, err
	}

	_, err = q.FindUserByEmail(ctx, inv.Email)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return Preview{}, fmt.Errorf("find account: %w", err)
	}

	status := currentStatus(inv.Status, inv.ExpiresAt, a.Clock.Now())

	return Preview{Invitation: inv, Status: status, HasAccount: err == nil}, nil
}

/*
Joined is the membership an accepted invitation gave, with its
workspace.
*/
type Joined struct {
	Member    models.WorkspaceMember
	Workspace models.Workspace
}

/*
AcceptInvitationInput is the signed-in user and the invitation link
they hold.
*/
type AcceptInvitationInput struct {
	User  models.User
	Token string
}

/*
AcceptInvitation turns an invitation into a membership of its
workspace.

Rules:
  - The link must belong to an invitation of a live workspace.
  - The user's address must be the invitation's, case aside.
  - The invitation must be pending and not expired.
  - An active member cannot accept another invitation to the workspace.
  - The user's address counts as verified from then on.

Events: [appevents.MemberJoined] for a new member or
[appevents.MemberReactivated] for one coming back, then
[appevents.InvitationAccepted].

Errors: [exceptions.ErrNotFound], [exceptions.ErrInvitationEmailMismatch],
[exceptions.ErrInvitationAlreadyAccepted], [exceptions.ErrInvitationRevoked],
[exceptions.ErrInvitationExpired], [exceptions.ErrAlreadyMember], or a
wrapped error.
*/
type AcceptInvitation struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	Mail   *fmail.Manager
	Links  appauth.Links
}

/*
Handle accepts the invitation and returns the membership it gave. Run
inside a transaction scoped to the invitation's workspace, it joins
that transaction, so an account created for the invitation and its
membership are made together.
*/
func (a *AcceptInvitation) Handle(ctx context.Context, in AcceptInvitationInput) (Joined, error) {
	found, err := findByToken(ctx, models.New(a.DB), in.Token)
	if err != nil {
		return Joined{}, err
	}

	if !strings.EqualFold(found.Email, in.User.Email) {
		return Joined{}, exceptions.ErrInvitationEmailMismatch
	}

	var joined Joined

	scoped := tenancy.With(ctx, found.WorkspaceID.String())
	err = a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		inv, err := a.lockPending(ctx, q, found)
		if err != nil {
			return err
		}

		if joined.Member, err = a.join(ctx, tx, inv, in.User); err != nil {
			return err
		}

		if joined.Workspace, err = q.FindWorkspace(ctx, inv.WorkspaceID); err != nil {
			return fmt.Errorf("find workspace: %w", err)
		}

		return a.finish(ctx, tx, acceptance{invitation: inv, user: in.User, joined: joined})
	})

	return joined, err
}

/*
lockPending locks the invitation and refuses one that cannot be
accepted any more.
*/
func (a *AcceptInvitation) lockPending(
	ctx context.Context, q *models.Queries, found models.PreviewInvitationRow,
) (models.Invitation, error) {
	inv, err := q.LockInvitation(ctx, models.LockInvitationParams{WorkspaceID: found.WorkspaceID, ID: found.ID})
	if err != nil {
		return inv, fmt.Errorf("lock invitation: %w", err)
	}

	switch currentStatus(inv.Status, inv.ExpiresAt, a.Clock.Now()) {
	case enums.InvitationStatusAccepted:
		return inv, exceptions.ErrInvitationAlreadyAccepted
	case enums.InvitationStatusRevoked:
		return inv, exceptions.ErrInvitationRevoked
	case enums.InvitationStatusExpired:
		return inv, exceptions.ErrInvitationExpired
	case enums.InvitationStatusPending:
		return inv, nil
	default:
		return inv, nil
	}
}

/*
join gives the user a membership in the invitation's role, bringing
back a deactivated one, and verifies their address.
*/
func (a *AcceptInvitation) join(
	ctx context.Context, tx *database.Tx, inv models.Invitation, user models.User,
) (models.WorkspaceMember, error) {
	q := models.New(tx)
	now := a.Clock.Now()

	if err := q.MarkUserEmailVerified(ctx, models.MarkUserEmailVerifiedParams{ID: user.ID, Now: now}); err != nil {
		return models.WorkspaceMember{}, fmt.Errorf("verify email: %w", err)
	}

	existing, err := q.FindWorkspaceMemberForUser(ctx, models.FindWorkspaceMemberForUserParams{
		WorkspaceID: inv.WorkspaceID, UserID: user.ID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return a.add(ctx, tx, inv, user)
	}

	if err != nil {
		return existing, fmt.Errorf("find membership: %w", err)
	}

	if existing.Status == string(enums.MemberStatusActive) {
		return existing, exceptions.ErrAlreadyMember
	}

	member, err := q.ReactivateWorkspaceMember(ctx, models.ReactivateWorkspaceMemberParams{
		WorkspaceID: inv.WorkspaceID, ID: existing.ID, Role: inv.Role, Now: now,
	})
	if err != nil {
		return member, fmt.Errorf("reactivate member: %w", err)
	}

	return member, a.Events.Dispatch(ctx, tx, appevents.MemberReactivated{
		WorkspaceID: inv.WorkspaceID, MemberID: member.ID, UserID: user.ID, Role: member.Role,
		ReactivatedBy: invitedBy(inv),
	})
}

/*
add creates the user's membership in the invitation's role.
*/
func (a *AcceptInvitation) add(
	ctx context.Context, tx *database.Tx, inv models.Invitation, user models.User,
) (models.WorkspaceMember, error) {
	member, err := models.New(tx).CreateWorkspaceMember(ctx, models.CreateWorkspaceMemberParams{
		WorkspaceID: inv.WorkspaceID, UserID: user.ID, Role: inv.Role, JoinedVia: new(appauth.MethodInvitation),
		Now: a.Clock.Now(),
	})
	if err != nil {
		return member, fmt.Errorf("add member: %w", err)
	}

	return member, a.Events.Dispatch(ctx, tx, appevents.MemberJoined{
		WorkspaceID: inv.WorkspaceID, MemberID: member.ID, UserID: user.ID, JoinedVia: appauth.MethodInvitation,
	})
}

/*
acceptance is an invitation being accepted: the invitation, the user
accepting it and the membership it gave.
*/
type acceptance struct {
	invitation models.Invitation
	user       models.User
	joined     Joined
}

/*
finish puts the new member in the invitation's teams, marks the
invitation accepted, and tells whoever sent it.
*/
func (a *AcceptInvitation) finish(ctx context.Context, tx *database.Tx, acc acceptance) error {
	q := models.New(tx)
	inv, user, joined := acc.invitation, acc.user, acc.joined
	now := a.Clock.Now()

	err := q.AddMemberToTeams(ctx, models.AddMemberToTeamsParams{
		WorkspaceID: inv.WorkspaceID, WorkspaceMemberID: joined.Member.ID, TeamIds: inv.TeamIds, Now: now,
	})
	if err != nil {
		return fmt.Errorf("join teams: %w", err)
	}

	_, err = q.AcceptInvitation(ctx, models.AcceptInvitationParams{
		WorkspaceID: inv.WorkspaceID, ID: inv.ID, UserID: &user.ID, Now: now,
	})
	if err != nil {
		return fmt.Errorf("accept invitation: %w", err)
	}

	err = a.Events.Dispatch(ctx, tx, appevents.InvitationAccepted{
		WorkspaceID: inv.WorkspaceID, InvitationID: inv.ID, MemberID: joined.Member.ID, UserID: user.ID,
	})
	if err != nil {
		return err
	}

	return a.tellInviter(ctx, tx, acc)
}

/*
tellInviter mails whoever sent the invitation, while they are still an
active member.
*/
func (a *AcceptInvitation) tellInviter(ctx context.Context, tx *database.Tx, acc acceptance) error {
	inv := acc.invitation
	if inv.InvitedByMemberID == nil {
		return nil
	}

	q := models.New(tx)

	inviter, err := q.FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
		WorkspaceID: inv.WorkspaceID, ID: *inv.InvitedByMemberID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find inviter: %w", err)
	}

	if inviter.WorkspaceMember.Status != string(enums.MemberStatusActive) {
		return nil
	}

	return appmail.ToUser(a.Mail, inviter.User).QueueTx(ctx, tx, appmail.InvitationAcceptedMail{
		Name: inviter.User.Name, Workspace: acc.joined.Workspace.Name, Member: acc.user.Name,
		MembersURL: a.Links.Members(),
	})
}

/*
findByToken returns the invitation the link belongs to, in a live
workspace.
*/
func findByToken(ctx context.Context, q *models.Queries, token string) (models.PreviewInvitationRow, error) {
	if token == "" {
		return models.PreviewInvitationRow{}, exceptions.ErrNotFound
	}

	inv, err := q.PreviewInvitation(ctx, appauth.Digest(token))
	if errors.Is(err, database.ErrNotFound) {
		return inv, exceptions.ErrNotFound
	}

	if err != nil {
		return inv, fmt.Errorf("find invitation: %w", err)
	}

	return inv, nil
}

/*
currentStatus returns where an invitation stands now: a pending one
past its expiry has expired.
*/
func currentStatus(stored string, expiresAt, now time.Time) enums.InvitationStatus {
	status := enums.InvitationStatus(stored)
	if status == enums.InvitationStatusPending && !now.Before(expiresAt) {
		return enums.InvitationStatusExpired
	}

	return status
}

/*
invitedBy returns the member who sent the invitation, ids.Nil() when
they are gone.
*/
func invitedBy(inv models.Invitation) ids.UUID {
	if inv.InvitedByMemberID == nil {
		return ids.Nil()
	}

	return *inv.InvitedByMemberID
}
