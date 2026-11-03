/*
|--------------------------------------------------------------------------
| List Teams
|--------------------------------------------------------------------------
|
| Everyone who can see the members of a workspace can see its teams and
| who is in each, since that is how they know where to send work. Teams
| come a page at a time by name, each with its member count; the members
| themselves come along only when asked for. Deleted teams never list.
|
*/

package teams

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ListTeamsInput is who asks, through their membership, which page of
teams they want, and whether each team's members come along.
*/
type ListTeamsInput struct {
	Membership  authz.Membership
	WithMembers bool
	Offset      int
	Limit       int
}

/*
TeamPage is one page of teams, and whether more follow.
*/
type TeamPage struct {
	Teams []Team
	More  bool
}

/*
ListTeams lists the live teams of the caller's workspace.

Rules:
  - The caller needs members:read.
  - Deleted teams are left out.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListTeams struct {
	DB *database.DB
}

/*
Handle returns the page of teams.
*/
func (a *ListTeams) Handle(ctx context.Context, in ListTeamsInput) (TeamPage, error) {
	if !in.Membership.Can(readPermission) {
		return TeamPage{}, exceptions.ErrForbidden
	}

	var page TeamPage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListTeams(ctx, models.ListTeamsParams{
			WorkspaceID: in.Membership.Workspace.ID, RowOffset: int64(in.Offset), RowLimit: int64(in.Limit + 1),
		})
		if err != nil {
			return fmt.Errorf("list teams: %w", err)
		}

		if len(rows) > in.Limit {
			rows, page.More = rows[:in.Limit], true
		}

		for _, row := range rows {
			page.Teams = append(page.Teams, Team{Team: row.Team, MemberCount: row.MemberCount})
		}

		if !in.WithMembers || len(page.Teams) == 0 {
			return nil
		}

		return loadMembers(ctx, q, in.Membership.Workspace.ID, page.Teams)
	})

	return page, err
}
