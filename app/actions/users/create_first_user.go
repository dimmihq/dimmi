/*
|--------------------------------------------------------------------------
| Create First User
|--------------------------------------------------------------------------
|
| A fresh self-hosted install has nobody in it, and the first person to
| arrive becomes the owner of its first workspace. The setup page and
| the user:create command both get there, possibly at the same moment,
| so both take the same lock and only the first one finds the install
| empty; the other is told setup is already done. The workspace's
| defaults are made once, with it.
|
*/

package users

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
firstUserLock names the lock every way of creating the first user
takes.
*/
const firstUserLock = "users:first-user"

/*
CreateFirstUserInput is the first person and their workspace: their
name, email and password hash, the workspace's name, and how they
arrived, such as setup or cli.
*/
type CreateFirstUserInput struct {
	Name          string
	Email         string
	PasswordHash  *string
	WorkspaceName string
	Via           string
}

/*
FirstUser is the person created and the workspace they own.
*/
type FirstUser struct {
	User      models.User
	Workspace workspaces.CreatedWorkspace
}

/*
CreateFirstUser creates the install's first account and its first
workspace, owned by it.

Rules:
  - Only an install with no live account gets a first user; the check
    and the creation hold one lock, so two at once cannot both win.
  - The account is verified: whoever runs the install vouches for it.

Errors: [exceptions.ErrSetupCompleted], or a wrapped error.
*/
type CreateFirstUser struct {
	DB              *database.DB
	CreateUser      *CreateUser
	CreateWorkspace *workspaces.CreateWorkspace
}

/*
Handle creates the account and its workspace in one transaction.
*/
func (a *CreateFirstUser) Handle(ctx context.Context, in CreateFirstUserInput) (FirstUser, error) {
	var out FirstUser

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if err := tx.AdvisoryLock(ctx, firstUserLock); err != nil {
			return err
		}

		count, err := models.New(tx).CountLiveUsers(ctx)
		if err != nil {
			return fmt.Errorf("count users: %w", err)
		}

		if count > 0 {
			return exceptions.ErrSetupCompleted
		}

		out.User, err = a.CreateUser.Handle(ctx, CreateUserInput{
			Name: in.Name, Email: in.Email, PasswordHash: in.PasswordHash, Verified: true, CreatedVia: in.Via,
		})
		if err != nil {
			return err
		}

		out.Workspace, err = a.CreateWorkspace.Handle(ctx, workspaces.CreateWorkspaceInput{
			Owner: out.User, Name: in.WorkspaceName, JoinedVia: in.Via,
		})

		return err
	})

	return out, err
}
