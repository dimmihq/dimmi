/*
|--------------------------------------------------------------------------
| Create And Rename Teams
|--------------------------------------------------------------------------
|
| Admins create teams and rename them. A name is kept in composed
| Unicode form without invisible characters, and two live teams of a
| workspace never share one regardless of letter case, so "Billing" and
| "billing" cannot both exist. A deleted team's name is free again at
| once.
|
*/

package teams

import (
	"context"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CreateTeamInput is who creates a team, through their membership, and
its name.
*/
type CreateTeamInput struct {
	Membership authz.Membership
	Name       string
}

/*
CreateTeam adds a team to the caller's workspace.

Rules:
  - The caller needs members:write.
  - The name is unique among the workspace's live teams, ignoring case.

Events: [appevents.TeamCreated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrTeamNameTaken], or a
wrapped database error.
*/
type CreateTeam struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle creates the team and returns it, without members.
*/
func (a *CreateTeam) Handle(ctx context.Context, in CreateTeamInput) (Team, error) {
	if !in.Membership.Can(writePermission) {
		return Team{}, exceptions.ErrForbidden
	}

	var team models.Team

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		team, err = models.New(tx).CreateTeam(ctx, models.CreateTeamParams{
			WorkspaceID: in.Membership.Workspace.ID, Name: str.Clean(in.Name), Now: a.Clock.Now(),
		})
		if err != nil {
			return nameError(err, "create team")
		}

		return a.Events.Dispatch(ctx, tx, appevents.TeamCreated{
			WorkspaceID: team.WorkspaceID, TeamID: team.ID, Name: team.Name, CreatedBy: in.Membership.Member.ID,
		})
	})
	if err != nil {
		return Team{}, err
	}

	return Team{Team: team}, nil
}

/*
UpdateTeamInput is who renames a team, through their membership, the
team and its new name.
*/
type UpdateTeamInput struct {
	Membership authz.Membership
	TeamID     ids.UUID
	Name       string
}

/*
UpdateTeam renames a live team of the caller's workspace.

Rules:
  - The caller needs members:write.
  - Deleted teams and teams of other workspaces are not found.
  - The name is unique among the workspace's live teams, ignoring case.
  - The current name changes nothing.

Events: [appevents.TeamUpdated] when the name changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrTeamNameTaken], or a wrapped database error.
*/
type UpdateTeam struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle renames the team and returns it as it now is.
*/
func (a *UpdateTeam) Handle(ctx context.Context, in UpdateTeamInput) (Team, error) {
	if !in.Membership.Can(writePermission) {
		return Team{}, exceptions.ErrForbidden
	}

	var out Team

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		workspaceID := in.Membership.Workspace.ID

		team, err := lockTeam(ctx, q, workspaceID, in.TeamID)
		if err != nil {
			return err
		}

		if err := a.rename(ctx, tx, team, in); err != nil {
			return err
		}

		out, err = findTeam(ctx, q, workspaceID, team.ID)

		return err
	})

	return out, err
}

/*
rename gives the team the name asked for and announces it, unless it
already has exactly that name once cleaned.
*/
func (a *UpdateTeam) rename(ctx context.Context, tx *database.Tx, team models.Team, in UpdateTeamInput) error {
	name := str.Clean(in.Name)
	if name == team.Name {
		return nil
	}

	renamed, err := models.New(tx).RenameTeam(ctx, models.RenameTeamParams{
		WorkspaceID: team.WorkspaceID, ID: team.ID, Name: name, Now: a.Clock.Now(),
	})
	if err != nil {
		return nameError(err, "rename team")
	}

	return a.Events.Dispatch(ctx, tx, appevents.TeamUpdated{
		WorkspaceID: team.WorkspaceID, TeamID: team.ID, From: team.Name, To: renamed.Name,
		UpdatedBy: in.Membership.Member.ID,
	})
}
