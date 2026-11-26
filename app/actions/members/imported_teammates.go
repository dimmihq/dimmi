/*
|--------------------------------------------------------------------------
| Imported Teammates
|--------------------------------------------------------------------------
|
| A teammate brought over from another tool waits, deactivated, until an
| admin enables them. Once they sign up and prove their address, by any
| way of proving it, the admins of every workspace they wait in hear it
| once, so nobody waits unseen.
|
| Enabling someone who has proven their address lets them in at once,
| with the role and teams the import remembered, and tells them. Someone
| who has not is mailed an invitation instead: only the owner of the inbox
| holds its link, so a stranger who signed up with their address first
| never gets in. Accepting it brings the membership back like any
| invitation of a deactivated member.
|
*/

package members

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
NoteImportArrivals tells admins that an imported teammate is waiting.

Rules:
  - Runs in the caller's transaction, which the context carries, once
    the user's address is proven.
  - Each workspace where the user waits to be enabled records that they
    arrived, the first time only, and mails its active owners and admins.

Errors: a wrapped database or mail error.
*/
type NoteImportArrivals struct {
	DB    *database.DB
	Clock clock.Clock
	Mail  *mail.Manager
	Links appauth.Links
}

/*
Handle records the user's arrival in every workspace they wait in.
*/
func (a *NoteImportArrivals) Handle(ctx context.Context, tx *database.Tx, user models.User) error {
	rows, err := models.New(tx).ListUserAwaitingMemberships(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("list awaiting memberships: %w", err)
	}

	for _, row := range rows {
		scoped := tenancy.With(ctx, row.WorkspaceID.String())

		err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
			return a.arrive(ctx, tx, row, user)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
arrive marks the user arrived in one workspace and mails its admins,
unless they arrived there before.
*/
func (a *NoteImportArrivals) arrive(
	ctx context.Context, tx *database.Tx, row models.ListUserAwaitingMembershipsRow, user models.User,
) error {
	q := models.New(tx)

	marked, err := q.MarkImportArrived(ctx, models.MarkImportArrivedParams{
		WorkspaceID: row.WorkspaceID, UserID: user.ID, Now: a.Clock.Now(),
	})
	if err != nil {
		return fmt.Errorf("mark import arrived: %w", err)
	}

	if len(marked) == 0 {
		return nil
	}

	managers, err := q.ListActiveManagers(ctx, row.WorkspaceID)
	if err != nil {
		return fmt.Errorf("list admins: %w", err)
	}

	for _, manager := range managers {
		err := appmail.ToUser(a.Mail, manager.User).QueueTx(ctx, tx, appmail.ImportedTeammateWaitingMail{
			Name: manager.User.Name, Workspace: row.WorkspaceName, Teammate: user.Name, Email: user.Email,
			MembersURL: a.Links.Members(),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
enabling reports whether the update enables an imported member who waits
to be enabled.
*/
func enabling(in UpdateMemberInput, target models.WorkspaceMember) bool {
	return target.ImportPending && target.Status == string(enums.MemberStatusDeactivated) &&
		in.Status != nil && *in.Status == enums.MemberStatusActive
}

/*
enable lets the imported member in, or invites them when they have not
proven their address yet. The role asked for, when one is, takes the
place of the one the import remembered; ownership is never given this
way. Nothing else in the update applies to a member who is still out.
*/
func (a *UpdateMember) enable(ctx context.Context, tx *database.Tx, s subject, in UpdateMemberInput) error {
	if !in.Membership.Can("members:write") {
		return exceptions.ErrForbidden
	}

	role := enums.Role(s.target.Role)
	if s.target.ImportRole != nil {
		role = enums.Role(*s.target.ImportRole)
	}

	if in.AssignedRole != nil {
		if appauth.IsOwnerRole(*in.AssignedRole) {
			return transition("role", s.target.Status, string(*in.AssignedRole))
		}

		role = *in.AssignedRole
	}

	user, err := s.q.FindUser(ctx, s.target.UserID)
	if err != nil {
		return fmt.Errorf("load imported member: %w", err)
	}

	if !user.IsVerified() {
		return a.inviteImported(ctx, s, in, invitations.Invitee{
			Email: user.Email, Role: role, TeamIDs: s.target.ImportTeamIds,
		})
	}

	w := memberWriter{tx: tx, events: a.Events, by: in.Membership.Member.ID, now: a.Clock.Now()}

	member, err := w.reactivate(ctx, s.target, role)
	if err != nil {
		return err
	}

	if err := w.joinTeams(ctx, member, s.target.ImportTeamIds); err != nil {
		return err
	}

	return appmail.ToUser(a.Mail, user).QueueTx(ctx, tx, appmail.AccessEnabledMail{
		Name: user.Name, Workspace: s.workspace.Name, AppURL: a.Links.App(),
	})
}

/*
inviteImported mails the imported member an invitation in the role and
teams they are enabled with, and records when it went.
*/
func (a *UpdateMember) inviteImported(
	ctx context.Context, s subject, in UpdateMemberInput, invitee invitations.Invitee,
) error {
	if a.Invite == nil {
		return errors.New("invite imported member: no invitations")
	}

	outcomes, err := a.Invite.Handle(ctx, invitations.CreateInvitationsInput{
		Membership: in.Membership, Inviter: in.Principal.User, Invitees: []invitations.Invitee{invitee},
	})
	if err != nil {
		return err
	}

	if len(outcomes) == 1 && outcomes[0].Err != nil {
		return outcomes[0].Err
	}

	return s.q.MarkImportInvited(ctx, models.MarkImportInvitedParams{
		WorkspaceID: s.target.WorkspaceID, ID: s.target.ID, Now: a.Clock.Now(),
	})
}

/*
joinTeams puts the member in each of the teams that still exist,
announcing each one they joined.
*/
func (w memberWriter) joinTeams(ctx context.Context, m models.WorkspaceMember, teams []ids.UUID) error {
	q := models.New(w.tx)

	live, err := q.ListExistingTeams(ctx, models.ListExistingTeamsParams{WorkspaceID: m.WorkspaceID, Ids: teams})
	if err != nil {
		return fmt.Errorf("find imported teams: %w", err)
	}

	for _, team := range live {
		added, err := q.AddTeamMember(ctx, models.AddTeamMemberParams{
			WorkspaceID: m.WorkspaceID, TeamID: team, WorkspaceMemberID: m.ID, Now: w.now,
		})
		if err != nil {
			return fmt.Errorf("join imported team: %w", err)
		}

		if added == 0 {
			continue
		}

		err = w.events.Dispatch(ctx, w.tx, appevents.TeamMemberAdded{
			WorkspaceID: m.WorkspaceID, TeamID: team, MemberID: m.ID, AddedByMemberID: w.by,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
