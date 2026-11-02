/*
|--------------------------------------------------------------------------
| Workspace Factory
|--------------------------------------------------------------------------
|
| Workspaces for tests, made by the same action as in production, so
| each comes with its organization and owner. Members join through the
| same insert the product uses.
|
*/

package factories

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Workspace creates a workspace the user owns.
*/
func Workspace(ctx context.Context, a *app.App, owner models.User) (workspaces.CreatedWorkspace, error) {
	return a.Actions.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
		Owner: owner, Name: "Acme " + owner.Name, JoinedVia: appauth.JoinedViaCreated,
	})
}

/*
Membership is a user joining a workspace with a role.
*/
type Membership struct {
	Workspace models.Workspace
	User      models.User
	Role      enums.Role
}

/*
Member adds the user to the workspace with the role, as accepting an
invitation does.
*/
func Member(ctx context.Context, a *app.App, m Membership) (models.WorkspaceMember, error) {
	var member models.WorkspaceMember

	scoped := tenancy.With(ctx, m.Workspace.ID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error

		member, err = models.New(tx).CreateWorkspaceMember(ctx, models.CreateWorkspaceMemberParams{
			WorkspaceID: m.Workspace.ID, UserID: m.User.ID, Role: string(m.Role),
			JoinedVia: new(appauth.MethodInvitation), Now: a.Clock.Now(),
		})

		return err
	})
	if err != nil {
		return models.WorkspaceMember{}, fmt.Errorf("add member: %w", err)
	}

	return member, nil
}

/*
Deactivate deactivates the member, the way removing them does.
*/
func Deactivate(ctx context.Context, a *app.App, m models.WorkspaceMember) error {
	scoped := tenancy.With(ctx, m.WorkspaceID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return models.New(tx).UpdateWorkspaceMemberStatus(ctx, models.UpdateWorkspaceMemberStatusParams{
			Status: string(enums.MemberStatusDeactivated), Now: a.Clock.Now(), WorkspaceID: m.WorkspaceID, ID: m.ID,
		})
	})
	if err != nil {
		return fmt.Errorf("deactivate member: %w", err)
	}

	return nil
}
