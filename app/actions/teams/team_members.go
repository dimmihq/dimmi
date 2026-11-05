/*
|--------------------------------------------------------------------------
| Team Membership
|--------------------------------------------------------------------------
|
| Admins put members in teams several at a time and take them out one at
| a time. Adding several never fails as a whole: each member gets their
| own result, so a typo or a member who left does not block the rest.
| Only active members join a team; a deactivated member is skipped and
| said so, since they no longer take work. The team row is locked while
| its members change, so a team deleted at the same moment never ends
| up with new members.
|
*/

package teams

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
What adding one member to a team came to: added, already in the team,
skipped because they are deactivated, or not a member of the workspace.
*/
const (
	ResultAdded         = "added"
	ResultAlreadyMember = "already_member"
	ResultInactive      = "inactive"
	ResultNotFound      = "not_found"
)

/*
AddTeamMembersInput is who adds members, through their membership, the
team, and the members to add in the order the results come back.
*/
type AddTeamMembersInput struct {
	Membership authz.Membership
	TeamID     ids.UUID
	MemberIDs  []ids.UUID
}

/*
AddTeamMembers puts members of the caller's workspace in one of its
live teams.

Rules:
  - The caller needs members:write.
  - Deleted teams and teams of other workspaces are not found.
  - Each member gets a result: ResultAdded, ResultAlreadyMember,
    ResultInactive for a deactivated member, who is skipped, or
    ResultNotFound for anyone outside the workspace.

Events: [appevents.TeamMemberAdded] for each member added.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type AddTeamMembers struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle adds the members and returns one result per member, in order.
*/
func (a *AddTeamMembers) Handle(ctx context.Context, in AddTeamMembersInput) ([]string, error) {
	if !in.Membership.Can(writePermission) {
		return nil, exceptions.ErrForbidden
	}

	results := make([]string, 0, len(in.MemberIDs))

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		team, err := lockTeam(ctx, models.New(tx), in.Membership.Workspace.ID, in.TeamID)
		if err != nil {
			return err
		}

		for _, memberID := range in.MemberIDs {
			result, err := a.add(ctx, models.New(tx), team, memberID)
			if err != nil {
				return err
			}

			results = append(results, result)

			if result != ResultAdded {
				continue
			}

			err = a.Events.Dispatch(ctx, tx, appevents.TeamMemberAdded{
				WorkspaceID: team.WorkspaceID, TeamID: team.ID, MemberID: memberID,
				AddedByMemberID: in.Membership.Member.ID,
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	return results, err
}

/*
add puts one member in the team and says what came of it.
*/
func (a *AddTeamMembers) add(
	ctx context.Context, q *models.Queries, team models.Team, memberID ids.UUID,
) (string, error) {
	member, err := q.FindWorkspaceMember(ctx, models.FindWorkspaceMemberParams{
		WorkspaceID: team.WorkspaceID, ID: memberID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return ResultNotFound, nil
	}

	if err != nil {
		return "", fmt.Errorf("find member: %w", err)
	}

	if member.Status != string(enums.MemberStatusActive) {
		return ResultInactive, nil
	}

	added, err := q.AddTeamMember(ctx, models.AddTeamMemberParams{
		WorkspaceID: team.WorkspaceID, TeamID: team.ID, WorkspaceMemberID: member.ID, Now: a.Clock.Now(),
	})
	if err != nil {
		return "", fmt.Errorf("add team member: %w", err)
	}

	if added == 0 {
		return ResultAlreadyMember, nil
	}

	return ResultAdded, nil
}

/*
RemoveTeamMemberInput is who takes a member out of a team, through their
membership, the team and the member.
*/
type RemoveTeamMemberInput struct {
	Membership authz.Membership
	TeamID     ids.UUID
	MemberID   ids.UUID
}

/*
RemoveTeamMember takes a member out of a live team of the caller's
workspace.

Rules:
  - The caller needs members:write.
  - Deleted teams, teams of other workspaces and members not in the
    team are not found.

Events: [appevents.TeamMemberRemoved].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type RemoveTeamMember struct {
	DB     *database.DB
	Events *events.Dispatcher
}

/*
Handle takes the member out of the team.
*/
func (a *RemoveTeamMember) Handle(ctx context.Context, in RemoveTeamMemberInput) error {
	if !in.Membership.Can(writePermission) {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		team, err := lockTeam(ctx, q, in.Membership.Workspace.ID, in.TeamID)
		if err != nil {
			return err
		}

		removed, err := q.RemoveTeamMember(ctx, models.RemoveTeamMemberParams{
			WorkspaceID: team.WorkspaceID, TeamID: team.ID, WorkspaceMemberID: in.MemberID,
		})
		if err != nil {
			return fmt.Errorf("remove team member: %w", err)
		}

		if removed == 0 {
			return exceptions.ErrNotFound
		}

		return a.Events.Dispatch(ctx, tx, appevents.TeamMemberRemoved{
			WorkspaceID: team.WorkspaceID, TeamID: team.ID, MemberID: in.MemberID, RemovedBy: in.Membership.Member.ID,
		})
	})
}
