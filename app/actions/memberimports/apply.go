/*
|--------------------------------------------------------------------------
| Member Import Changes
|--------------------------------------------------------------------------
|
| A checked row changes a member through the same actions as a change
| made by hand, so it meets the same rules, fires the same events and
| lands in the audit log the same way. An address that is not a member
| is invited instead. A dry run stops right before either.
|
*/

package memberimports

import (
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/invitations"
	"github.com/dimmihq/dimmi/app/actions/members"
	"github.com/dimmihq/dimmi/app/actions/teams"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
job is one checked row to run: the import it belongs to, the row and
what the row wants.
*/
type job struct {
	in   ImportMembersInput
	row  row
	want wanted
}

/*
moves are the changes a row makes to a member: the role and status,
nil for each it leaves alone, and the teams to join and to leave.
*/
type moves struct {
	role   *enums.Role
	status *enums.MemberStatus
	join   []ids.UUID
	leave  []ids.UUID
}

/*
change applies the row to the member and reports which fields moved.
An owner keeps their role and status; their teams may change. A row
that deactivates the member leaves teams alone, since deactivating
already takes the member out of every team.
*/
func (a *ImportMembers) change(ctx context.Context, j job, member models.WorkspaceMember) (RowReport, error) {
	current, err := a.currentTeams(ctx, j.in, member.ID)
	if err != nil {
		return RowReport{}, err
	}

	m := moved(j.want, member)
	if ownerChange(member, m) {
		return failed(j.row, exceptions.ErrImportOwnerNotChanged)
	}

	if m.status != nil && *m.status == enums.MemberStatusDeactivated {
		j.want.teams = nil
	}

	m.join, m.leave = teamChanges(j.want.teams, current)
	changes := fieldsChanged(m.role != nil, m.status != nil, len(m.join)+len(m.leave) > 0)
	report := RowReport{Row: j.row.number, Email: j.want.email, Result: ResultUpdated, Changes: changes}

	if len(changes) == 0 {
		report.Result = ResultUnchanged

		return report, nil
	}

	if j.in.DryRun {
		return report, nil
	}

	if err := a.apply(ctx, j.in, member.ID, m); err != nil {
		return failed(j.row, err)
	}

	return report, nil
}

/*
ownerChange reports whether the row would change an owner's role or
status.
*/
func ownerChange(member models.WorkspaceMember, m moves) bool {
	held := member.Role

	return appauth.IsOwnerRole(enums.Role(held)) && (m.role != nil || m.status != nil)
}

/*
moved returns the role and status the row changes, nil for each it
leaves as the member has it.
*/
func moved(want wanted, member models.WorkspaceMember) moves {
	m := moves{role: want.role, status: want.status}
	held := member.Role

	if m.role != nil && string(*m.role) == held {
		m.role = nil
	}

	if m.status != nil && string(*m.status) == member.Status {
		m.status = nil
	}

	return m
}

/*
teamChanges returns the teams to join and to leave for the member to be
in exactly the wanted ones; nil wanted teams change nothing.
*/
func teamChanges(wantedTeams, current []ids.UUID) ([]ids.UUID, []ids.UUID) {
	if wantedTeams == nil {
		return nil, nil
	}

	var join, leave []ids.UUID

	for _, id := range wantedTeams {
		if !slices.Contains(current, id) {
			join = append(join, id)
		}
	}

	for _, id := range current {
		if !slices.Contains(wantedTeams, id) {
			leave = append(leave, id)
		}
	}

	return join, leave
}

/*
fieldsChanged names the fields a row changes, in a fixed order.
*/
func fieldsChanged(role, status, inTeams bool) []string {
	var out []string

	for _, field := range []struct {
		name    string
		changed bool
	}{{"role", role}, {"status", status}, {"teams", inTeams}} {
		if field.changed {
			out = append(out, field.name)
		}
	}

	return out
}

/*
currentTeams returns the ids of the teams the member is in.
*/
func (a *ImportMembers) currentTeams(ctx context.Context, in ImportMembersInput, id ids.UUID) ([]ids.UUID, error) {
	var out []ids.UUID

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		rows, err := models.New(tx).ListMemberTeams(ctx, models.ListMemberTeamsParams{
			WorkspaceID: in.Membership.Workspace.ID, MemberIds: []ids.UUID{id},
		})
		for _, row := range rows {
			out = append(out, row.Team.ID)
		}

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list member teams: %w", err)
	}

	return out, nil
}

/*
apply changes the member's role and status when the row moves either,
then puts them in the teams to join and out of those to leave.
*/
func (a *ImportMembers) apply(ctx context.Context, in ImportMembersInput, id ids.UUID, m moves) error {
	if m.role != nil || m.status != nil {
		_, err := a.Update.Handle(ctx, members.UpdateMemberInput{
			Principal: in.Principal, Membership: in.Membership, MemberID: id, AssignedRole: m.role, Status: m.status,
		})
		if err != nil {
			return err
		}
	}

	for _, team := range m.join {
		_, err := a.Add.Handle(ctx, teams.AddTeamMembersInput{
			Membership: in.Membership, TeamID: team, MemberIDs: []ids.UUID{id},
		})
		if err != nil {
			return err
		}
	}

	for _, team := range m.leave {
		err := a.Remove.Handle(ctx, teams.RemoveTeamMemberInput{Membership: in.Membership, TeamID: team, MemberID: id})
		if err != nil {
			return err
		}
	}

	return nil
}

/*
inviteAddress asks an address that is not a member to join, with the
row's role, a member by default, and its teams.
*/
func (a *ImportMembers) inviteAddress(ctx context.Context, j job) (RowReport, error) {
	in, r, want := j.in, j.row, j.want

	if want.status != nil && *want.status == enums.MemberStatusDeactivated {
		return failed(r, exceptions.ErrImportUnknownMember)
	}

	report := RowReport{Row: r.number, Email: want.email, Result: ResultCreated}
	if in.DryRun {
		return report, nil
	}

	role := enums.RoleMember
	if want.role != nil {
		role = *want.role
	}

	outcomes, err := a.Invite.Handle(ctx, invitations.CreateInvitationsInput{
		Membership: in.Membership, Inviter: in.Principal.User,
		Invitees: []invitations.Invitee{{Email: want.email, Role: role, TeamIDs: want.teams}},
	})
	if err != nil {
		return failed(r, err)
	}

	if len(outcomes) == 1 && outcomes[0].Err != nil {
		return failed(r, outcomes[0].Err)
	}

	return report, nil
}
