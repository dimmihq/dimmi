/*
|--------------------------------------------------------------------------
| Team Factory
|--------------------------------------------------------------------------
|
| Teams for tests, written with the same queries the product uses, so a
| team member always carries its team's workspace.
|
*/

package factories

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/models"
)

/*
TeamOf is a team to create: its workspace, its name and its members.
*/
type TeamOf struct {
	Workspace models.Workspace
	Name      string
	Members   []models.WorkspaceMember
}

/*
Team creates the team and puts its members in it.
*/
func Team(ctx context.Context, a *app.App, t TeamOf) (models.Team, error) {
	var team models.Team

	w := t.Workspace

	err := a.DB.Transaction(tenancy.With(ctx, w.ID.String()), func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		var err error

		team, err = q.CreateTeam(ctx, models.CreateTeamParams{WorkspaceID: w.ID, Name: t.Name, Now: a.Clock.Now()})
		if err != nil {
			return err
		}

		for _, m := range t.Members {
			_, err := q.AddTeamMember(ctx, models.AddTeamMemberParams{
				WorkspaceID: w.ID, TeamID: team.ID, WorkspaceMemberID: m.ID, Now: a.Clock.Now(),
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return models.Team{}, fmt.Errorf("create team: %w", err)
	}

	return team, nil
}
