/*
|--------------------------------------------------------------------------
| Team Lookups
|--------------------------------------------------------------------------
|
| The reads every team action shares. A team is only ever found inside
| the caller's workspace and only while it is live: a deleted team, or
| one of another workspace, simply does not exist to the caller. Changes
| lock the team row first, so a team cannot be deleted halfway through a
| change to it.
|
*/

package teams

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Permissions the team actions check: anyone who may see the members may
see the teams, and whoever may change the members may change the teams.
*/
const (
	readPermission  = "members:read"
	writePermission = "members:write"
)

/*
nameConstraint is the unique index that keeps live team names apart.
*/
const nameConstraint = "teams_workspace_id_name_key"

/*
Team is a live team with how many members it has. Members lists them
with their accounts, by name, when they were asked for, and is nil
otherwise. ConversationCounts counts the conversations it holds by
status, when asked for.
*/
type Team struct {
	Team               models.Team
	MemberCount        int64
	Members            []models.ListTeamMembersRow
	ConversationCounts *models.CountTeamConversationsByStatusRow
}

/*
lockTeam returns the live team and holds its row until the transaction
ends, or exceptions.ErrNotFound.
*/
func lockTeam(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.Team, error) {
	team, err := q.LockTeam(ctx, models.LockTeamParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return team, exceptions.ErrNotFound
	}

	if err != nil {
		return team, fmt.Errorf("lock team: %w", err)
	}

	return team, nil
}

/*
findTeam returns the live team with its member count, or
exceptions.ErrNotFound.
*/
func findTeam(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (Team, error) {
	row, err := q.FindTeam(ctx, models.FindTeamParams{WorkspaceID: workspaceID, ID: id})
	if errors.Is(err, database.ErrNotFound) {
		return Team{}, exceptions.ErrNotFound
	}

	if err != nil {
		return Team{}, fmt.Errorf("find team: %w", err)
	}

	return Team{Team: row.Team, MemberCount: row.MemberCount}, nil
}

/*
loadMembers fills in the members of each team, an empty list for a
team without any.
*/
func loadMembers(ctx context.Context, q *models.Queries, workspaceID ids.UUID, teams []Team) error {
	teamIDs := make([]ids.UUID, 0, len(teams))
	byID := make(map[ids.UUID]int, len(teams))

	for i := range teams {
		teamIDs = append(teamIDs, teams[i].Team.ID)
		byID[teams[i].Team.ID] = i
		teams[i].Members = []models.ListTeamMembersRow{}
	}

	rows, err := q.ListTeamMembers(ctx, models.ListTeamMembersParams{WorkspaceID: workspaceID, TeamIds: teamIDs})
	if err != nil {
		return fmt.Errorf("list team members: %w", err)
	}

	for _, row := range rows {
		i := byID[row.TeamID]
		teams[i].Members = append(teams[i].Members, row)
	}

	return nil
}

/*
nameError turns a clash on the unique team name into
exceptions.ErrTeamNameTaken and wraps any other failure with what was
being attempted.
*/
func nameError(err error, attempt string) error {
	if database.IsUniqueViolation(err, nameConstraint) {
		return exceptions.ErrTeamNameTaken
	}

	return fmt.Errorf("%s: %w", attempt, err)
}
