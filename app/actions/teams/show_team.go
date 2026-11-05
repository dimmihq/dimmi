/*
|--------------------------------------------------------------------------
| Show Teams
|--------------------------------------------------------------------------
|
| One team at a time, with its members when asked for, and the other way
| round: the teams each member belongs to, so a member's profile can say
| where they take work. Only live teams of the caller's workspace show.
|
*/

package teams

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ShowTeamInput is who asks, through their membership, the team they ask
about, and whether its members and its conversation counts come along.
*/
type ShowTeamInput struct {
	Membership             authz.Membership
	TeamID                 ids.UUID
	WithMembers            bool
	WithConversationCounts bool
}

/*
ShowTeam returns one live team of the caller's workspace.

Rules:
  - The caller needs members:read, and members:write for conversation
    counts, which count every conversation the team holds whoever may
    read it, as deleting the team acts on all of them.
  - Deleted teams and teams of other workspaces are not found.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowTeam struct {
	DB *database.DB
}

/*
Handle returns the team.
*/
func (a *ShowTeam) Handle(ctx context.Context, in ShowTeamInput) (Team, error) {
	if !in.Membership.Can(readPermission) || (in.WithConversationCounts && !in.Membership.Can(writePermission)) {
		return Team{}, exceptions.ErrForbidden
	}

	team := make([]Team, 1)

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error

		team[0], err = findTeam(ctx, q, in.Membership.Workspace.ID, in.TeamID)
		if err != nil {
			return err
		}

		if in.WithConversationCounts {
			counts, err := q.CountTeamConversationsByStatus(ctx, models.CountTeamConversationsByStatusParams{
				WorkspaceID: in.Membership.Workspace.ID, TeamID: in.TeamID,
			})
			if err != nil {
				return fmt.Errorf("count team conversations: %w", err)
			}

			team[0].ConversationCounts = &counts
		}

		if !in.WithMembers {
			return nil
		}

		return loadMembers(ctx, q, in.Membership.Workspace.ID, team)
	})

	return team[0], err
}

/*
ListMemberTeamsInput is who asks, through their membership, and the
members whose teams they want.
*/
type ListMemberTeamsInput struct {
	Membership authz.Membership
	MemberIDs  []ids.UUID
}

/*
ListMemberTeams returns the live teams each member belongs to, by name.
A member in no team has no entry.

Rules:
  - The caller needs members:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListMemberTeams struct {
	DB *database.DB
}

/*
Handle returns the teams of each member, keyed by member id.
*/
func (a *ListMemberTeams) Handle(ctx context.Context, in ListMemberTeamsInput) (map[ids.UUID][]models.Team, error) {
	if !in.Membership.Can(readPermission) {
		return nil, exceptions.ErrForbidden
	}

	var rows []models.ListMemberTeamsRow

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		rows, err = models.New(tx).ListMemberTeams(ctx, models.ListMemberTeamsParams{
			WorkspaceID: in.Membership.Workspace.ID, MemberIds: in.MemberIDs,
		})

		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list member teams: %w", err)
	}

	teams := make(map[ids.UUID][]models.Team, len(in.MemberIDs))
	for _, row := range rows {
		teams[row.WorkspaceMemberID] = append(teams[row.WorkspaceMemberID], row.Team)
	}

	return teams, nil
}
